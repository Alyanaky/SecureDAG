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
	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
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

	return &BadgerStore{
		db:         db,
		publicKey:  pubKey,
		privateKey: privKey,
	}, nil
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}

func (s *BadgerStore) PutBlock(data []byte) (string, error) {
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
		return txn.Set([]byte("key_"+hash), encryptedKey)
	})

	s.updateMetadata(hash, 1)
	return hash, err
}

func (s *BadgerStore) updateMetadata(hash string, delta int) error {
	return s.db.Update(func(txn *badger.Txn) error {
		metaKey := "meta_" + hash
		var meta BlockMetadata
		
		item, err := txn.Get([]byte(metaKey))
		if err == nil {
			val, _ := item.ValueCopy(nil)
			json.Unmarshal(val, &meta)
		}
		
		meta.References += delta
		meta.CreatedAt = time.Now()
		metaBytes, _ := json.Marshal(meta)
		return txn.Set([]byte(metaKey), metaBytes)
	})
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
		item, err := txn.Get([]byte("key_"+hash))
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
