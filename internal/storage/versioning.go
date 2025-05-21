package storage

import (
	"encoding/json"
	"fmt"
	"time"
)

type ObjectVersion struct {
	VersionID  string
	IsLatest   bool
	Modified   time.Time
	Size       int64
	ETag       string
	StorageClass string
}

func (s *BadgerStore) EnableVersioning(bucket string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(fmt.Sprintf("versioning_%s", bucket)), []byte{1})
	})
}

func (s *BadgerStore) PutObjectVersion(bucket, key string, data []byte, meta map[string]string) (ObjectVersion, error) {
	versionID := generateVersionID()
	
	version := ObjectVersion{
		VersionID:  versionID,
		Modified:   time.Now().UTC(),
		Size:       int64(len(data)),
		ETag:       fmt.Sprintf("%x", md5.Sum(data)),
		StorageClass: "STANDARD",
	}

	versionKey := fmt.Sprintf("versions/%s/%s/%s", bucket, key, versionID)
	versionData, _ := json.Marshal(version)

	err := s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set([]byte(versionKey), data); err != nil {
			return err
		}
		return txn.Set([]byte(versionKey+"/meta"), versionData)
	})

	return version, err
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
				item.Value(func(val []byte) error {
					return json.Unmarshal(val, &v)
				})
				versions = append(versions, v)
			}
		}
		return nil
	})

	return versions, err
}

func generateVersionID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
