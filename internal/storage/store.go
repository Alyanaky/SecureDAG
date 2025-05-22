package storage

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
)

var (
	ErrBucketNotFound = errors.New("bucket not found")
	ErrObjectNotFound = errors.New("object not found")
)

type ObjectVersion struct {
	VersionID    string    `json:"versionId"`
	Bucket       string    `json:"bucket"`
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
	ETag         string    `json:"etag"`
	StorageClass string    `json:"storageClass"`
	IsLatest     bool      `json:"isLatest"`
}

type Store interface {
	CreateBucket(name string) error
	DeleteBucket(name string) error
	PutObjectVersion(bucket, key string, data []byte, meta map[string]string) (ObjectVersion, error)
	GetObject(bucket, key, versionID string) ([]byte, ObjectVersion, error)
	ListObjectVersions(bucket, key string) ([]ObjectVersion, error)
	DeleteVersion(bucket, key, versionID string) error
	IterateObjects(bucket, prefix string, handler func(ObjectVersion) error) error
}

type BadgerStore struct {
	db *badger.DB
}

func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{db: db}, nil
}

func (s *BadgerStore) CreateBucket(bucket string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("bucket_"+bucket), []byte{})
	})
}

func (s *BadgerStore) PutObjectVersion(bucket, key string, data []byte, meta map[string]string) (ObjectVersion, error) {
	if !s.bucketExists(bucket) {
		return ObjectVersion{}, ErrBucketNotFound
	}

	versionID := fmt.Sprintf("%x", md5.Sum(data))
	version := ObjectVersion{
		VersionID:    versionID,
		Bucket:       bucket,
		Key:          key,
		Size:         int64(len(data)),
		LastModified: time.Now().UTC(),
		ETag:         versionID,
		StorageClass: "STANDARD",
		IsLatest:     true,
	}

	versionKey := objectVersionKey(bucket, key, versionID)
	versionData, _ := json.Marshal(version)

	err := s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set([]byte(versionKey), data); err != nil {
			return err
		}
		return txn.Set([]byte(versionKey+"/meta"), versionData)
	})

	return version, err
}

func (s *BadgerStore) GetObject(bucket, key, versionID string) ([]byte, ObjectVersion, error) {
	versionKey := objectVersionKey(bucket, key, versionID)
	var data []byte
	var version ObjectVersion

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(versionKey))
		if err != nil {
			return err
		}
		
		data, err = item.ValueCopy(nil)
		if err != nil {
			return err
		}

		metaItem, err := txn.Get([]byte(versionKey + "/meta"))
		if err != nil {
			return err
		}
		
		return metaItem.Value(func(meta []byte) error {
			return json.Unmarshal(meta, &version)
		})
	})

	return data, version, err
}

func (s *BadgerStore) ListObjectVersions(bucket, key string) ([]ObjectVersion, error) {
	var versions []ObjectVersion
	prefix := []byte(fmt.Sprintf("versions/%s/%s/", bucket, key))

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			if bytes.HasSuffix(item.Key(), []byte("/meta")) {
				var v ObjectVersion
				err := item.Value(func(val []byte) error {
					return json.Unmarshal(val, &v)
				})
				if err != nil {
					return err
				}
				versions = append(versions, v)
			}
		}
		return nil
	})

	return versions, err
}

func (s *BadgerStore) DeleteVersion(bucket, key, versionID string) error {
	versionKey := objectVersionKey(bucket, key, versionID)
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Delete([]byte(versionKey)); err != nil {
			return err
		}
		return txn.Delete([]byte(versionKey + "/meta"))
	})
}

func (s *BadgerStore) IterateObjects(bucket, prefix string, handler func(ObjectVersion) error) error {
	keyPrefix := fmt.Sprintf("versions/%s/%s/", bucket, prefix)
	return s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek([]byte(keyPrefix)); it.ValidForPrefix([]byte(keyPrefix)); it.Next() {
			item := it.Item()
			if bytes.HasSuffix(item.Key(), []byte("/meta")) {
				var v ObjectVersion
				err := item.Value(func(val []byte) error {
					return json.Unmarshal(val, &v)
				})
				if err != nil {
					return err
				}
				if err := handler(v); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// Helpers
func (s *BadgerStore) bucketExists(bucket string) bool {
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte("bucket_" + bucket))
		return err
	})
	return err == nil
}

func objectVersionKey(bucket, key, versionID string) string {
	return fmt.Sprintf("versions/%s/%s/%s", bucket, key, versionID)
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}
