package storage

import (
	"context"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/p2p"
	"github.com/libp2p/go-libp2p-kad-dht"
)

type Replicator struct {
	store    *BadgerStore
	dht      *dht.IpfsDHT
	interval time.Duration
}

func NewReplicator(store *BadgerStore, dht *dht.IpfsDHT, interval time.Duration) *Replicator {
	return &Replicator{
		store:    store,
		dht:      dht,
		interval: interval,
	}
}

func (r *Replicator) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	for {
		select {
		case <-ticker.C:
			r.checkAndRepair()
		case <-ctx.Done():
			return
		}
	}
}

func (r *Replicator) checkAndRepair() {
	r.store.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			if bytes.HasPrefix(key, []byte("meta_")) {
				var meta BlockMetadata
				item.Value(func(val []byte) error {
					json.Unmarshal(val, &meta)
					if meta.References < 3 {
						go r.replicateBlock(key[5:])
					}
					return nil
				})
			}
		}
		return nil
	})
}

func (r *Replicator) replicateBlock(hash []byte) {
	data, _ := r.store.GetBlock(string(hash))
	p2p.ReplicateBlock(context.Background(), r.dht, string(hash), data, 3)
}
