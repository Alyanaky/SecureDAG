package storage

import (
	"context"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/p2p"
)

const (
	HealInterval = 1 * time.Hour
	MinReplicas  = 3
)

func (s *BadgerStore) StartSelfHeal(ctx context.Context, dht *dht.IpfsDHT) {
	ticker := time.NewTicker(HealInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.checkAndHealBlocks(dht)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *BadgerStore) checkAndHealBlocks(dht *dht.IpfsDHT) {
	s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			if !bytes.HasPrefix(key, []byte("meta_")) {
				continue
			}

			var meta BlockMetadata
			item.Value(func(val []byte) error {
				json.Unmarshal(val, &meta)
				if meta.References < MinReplicas {
					go s.healBlock(string(key[5:]), dht)
				}
				return nil
			})
		}
		return nil
	})
}

func (s *BadgerStore) healBlock(hash string, dht *dht.IpfsDHT) {
	data, _ := s.GetBlock(hash)
	p2p.ReplicateBlock(context.Background(), dht, hash, data, MinReplicas)
}
