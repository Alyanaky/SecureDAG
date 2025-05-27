package storage

import (
    "context"
    "time"

    "github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) DeleteObjectVersion(ctx context.Context, bucket, key, versionID string) error {
    return s.db.Update(func(txn *badger.Txn) error {
        versionKey := []byte(bucket + "/" + key + "/" + versionID)
        return txn.Delete(versionKey)
    })
}
