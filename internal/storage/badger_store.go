package storage

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type BadgerStore struct {
	db *badger.DB
}

func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil // Отключаем логгер для чистоты вывода
	
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	
	return &BadgerStore{db: db}, nil
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}

func (s *BadgerStore) PutBlock(data []byte) (string, error) {
	hash := sha256.Sum256(data)
	key := fmt.Sprintf("%x", hash)
	
	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})
	
	return key, err
}

func (s *BadgerStore) GetBlock(hash string) ([]byte, error) {
	var valCopy []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(hash))
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			valCopy = append([]byte{}, val...)
			return nil
		})
	})
	
	return valCopy, err
}

func (s *BadgerStore) SplitAndStore(r io.Reader, chunkSize int) ([]string, error) {
	var hashes []string
	buf := make([]byte, chunkSize)
	
	for {
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}

		chunk := make([]byte, n)
		copy(chunk, buf[:n])
		
		hash, err := s.PutBlock(chunk)
		if err != nil {
			return nil, err
		}
		
		hashes = append(hashes, hash)
		
		if err == io.EOF {
			break
		}
	}
	
	return hashes, nil
}
