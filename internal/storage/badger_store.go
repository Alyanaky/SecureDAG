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
	"sync"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/crypto"
	"github.com/Alyanaky/SecureDAG/internal/p2p"
	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	"github.com/libp2p/go-libp2p-kad-dht"
)

const (
	HealInterval    = 1 * time.Hour
	MinReplicas     = 3
	MetadataPrefix  = "meta_"
	KeyPrefix       = "key_"
	QuotaPrefix     = "quota_"
	DeletionTimeout = 5 * time.Minute
)

type BlockMetadata struct {
	References  int                    `json:"refs"`
	CreatedAt   time.Time              `json:"created"`
	Size        int                    `json:"size"`
	S3Metadata  map[string]string      `json:"s3_meta,omitempty"`
	CustomTags  map[string]interface{} `json:"tags,omitempty"`
	ReplicaList []string               `json:"replicas"`
}

type BadgerStore struct {
	db          *badger.DB
	publicKey   *rsa.PublicKey
	privateKey  *rsa.PrivateKey
	mu          sync.RWMutex
	deletionMap map[string]chan struct{}
	quotaCache  map[string]int64
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
		db:          db,
		publicKey:   pubKey,
		privateKey:  privKey,
		deletionMap: make(map[string]chan struct{}),
		quotaCache:  make(map[string]int64),
	}

	go store.autoHeal()
	go crypto.KeyManager().StartRotation(context.Background())
	return store, nil
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}

func (s *BadgerStore) PutBlock(data []byte, meta map[string]string) (string, error) {
	contentHash := sha256.Sum256(data)
	existing, _ := s.GetBlock(fmt.Sprintf("%x", contentHash))
	if existing != nil {
		return fmt.Sprintf("%x", contentHash), nil
	}

	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		return "", err
	}

	encryptedData, err := crypto.EncryptData(data, aesKey)
	if err != nil {
		return "", err
	}

	encryptedKey, err := crypto.EncryptAESKey(s.publicKey, aesKey)
	if err != nil {
		return "", err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(encryptedData))
	err = s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set([]byte(hash), encryptedData); err != nil {
			return err
		}
		if err := txn.Set([]byte(KeyPrefix+hash), encryptedKey); err != nil {
			return err
		}
		return s.updateMetadata(txn, hash, meta, len(data))
	})

	return hash, err
}

func (s *BadgerStore) GetBlock(hash string) ([]byte, error) {
	var encryptedData, encryptedKey []byte

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(hash))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			encryptedData = append([]byte{}, val...)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(KeyPrefix + hash))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			encryptedKey = append([]byte{}, val...)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	aesKey, err := crypto.DecryptAESKey(s.privateKey, encryptedKey)
	if err != nil {
		return nil, err
	}

	return crypto.DecryptData(encryptedData, aesKey)
}

func (s *BadgerStore) GetBlockMetadata(hash string) (*BlockMetadata, error) {
	var meta BlockMetadata
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(MetadataPrefix + hash))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &meta)
		})
	})
	return &meta, err
}

func (s *BadgerStore) DeleteBlock(hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan struct{})
	s.deletionMap[hash] = ch

	go func() {
		select {
		case <-time.After(DeletionTimeout):
			s.forceDelete(hash)
		case <-ch:
			delete(s.deletionMap, hash)
		}
	}()

	return nil
}

func (s *BadgerStore) CancelDeletion(hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, exists := s.deletionMap[hash]; exists {
		close(ch)
		delete(s.deletionMap, hash)
	}
}

func (s *BadgerStore) forceDelete(hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.db.Update(func(txn *badger.Txn) error {
		txn.Delete([]byte(hash))
		txn.Delete([]byte(KeyPrefix+hash))
		txn.Delete([]byte(MetadataPrefix+hash))
		return nil
	})
}

func (s *BadgerStore) autoHeal() {
	ticker := time.NewTicker(HealInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				key := item.Key()
				if bytes.HasPrefix(key, []byte(MetadataPrefix)) {
					var meta BlockMetadata
					item.Value(func(val []byte) error {
						json.Unmarshal(val, &meta)
						if meta.References < MinReplicas {
							go s.healBlock(string(key[len(MetadataPrefix):]))
						}
						return nil
					})
				}
			}
			return nil
		})
	}
}

func (s *BadgerStore) healBlock(hash string) {
	data, err := s.GetBlock(hash)
	if err != nil {
		return
	}

	meta, _ := s.GetBlockMetadata(hash)
	nodes := p2p.SelectNodesForReplication(MinReplicas - meta.References)
	p2p.ReplicateToNodes(hash, data, nodes)
}

func (s *BadgerStore) updateMetadata(txn *badger.Txn, hash string, meta map[string]string, size int) error {
	var existing BlockMetadata
	item, err := txn.Get([]byte(MetadataPrefix + hash))
	if err == nil {
		item.Value(func(val []byte) error {
			return json.Unmarshal(val, &existing)
		})
	}

	existing.References++
	existing.Size = size
	existing.CreatedAt = time.Now().UTC()
	existing.S3Metadata = meta

	metaBytes, err := json.Marshal(existing)
	if err != nil {
		return err
	}

	return txn.Set([]byte(MetadataPrefix+hash), metaBytes)
}

func (s *BadgerStore) UpdateQuota(userID string, quota int64) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(QuotaPrefix+userID), []byte(fmt.Sprintf("%d", quota)))
	})
}

func (s *BadgerStore) GetQuota(userID string) (int64, error) {
	var quota int64
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(QuotaPrefix + userID))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			_, err := fmt.Sscanf(string(val), "%d", &quota)
			return err
		})
	})
	return quota, err
}
