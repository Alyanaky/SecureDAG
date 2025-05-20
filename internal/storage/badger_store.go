package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/crypto"
	"github.com/Alyanaky/SecureDAG/internal/p2p"
	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	"github.com/libp2p/go-libp2p-kad-dht"
)

const (
	HealInterval = 1 * time.Hour
	MinReplicas  = 3
)

type BlockMetadata struct {
	References int       `json:"refs"`
	CreatedAt  time.Time `json:"created"`
	Size       int       `json:"size"`
}

type BadgerStore struct {
	db         *badger.DB
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
}

func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil
	opts.Compression = options.ZSTD
	opts.IndexCacheSize = 256 << 20
	opts.WithCompactL0OnClose = true

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	privKey, pubKey, err := crypto.GenerateRSAKeys()
	if err != nil {
		return nil, err
	}

	store := &BadgerStore{
		db:         db,
		publicKey:  pubKey,
		privateKey: privKey,
	}

	go crypto.GetKeyManager().StartRotation(context.Background())
	return store, nil
}

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
			if bytes.HasPrefix(key, []byte("meta_")) {
				var meta BlockMetadata
				item.Value(func(val []byte) error {
					json.Unmarshal(val, &meta)
					if meta.References < MinReplicas {
						go s.healBlock(string(key[5:]), dht)
					}
					return nil
				})
			}
		}
		return nil
	})
}

func (s *BadgerStore) healBlock(hash string, dht *dht.IpfsDHT) {
	data, _ := s.GetBlock(hash)
	p2p.ReplicateBlock(context.Background(), dht, hash, data, MinReplicas)
}

// ... остальные методы остаются без изменений ...
