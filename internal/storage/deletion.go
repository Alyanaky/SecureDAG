package storage

import (
    "context"
    "time"

    "github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) SoftDeleteObject(ctx context.Context, bucket, key string) error {
    return s.db.Update(func(txn *badger.Txn) error {
        objKey := []byte(bucket + "/" + key)
        deletedKey := []byte(bucket + "/" + key + "/deleted")
        item, err := txn.Get(objKey)
        if err != nil {
            return err
        }
        data, err := item.ValueCopy(nil)
        if err != nil {
            return err
        }
        if err := txn.Set(deletedKey, data); err != nil {
            return err
        }
        return txn.Delete(objKey)
    })
}

func (s *BadgerStore) PurgeDeletedObjects(ctx context.Context, retention time.Duration) error {
    cutoff := time.Now().Add(-retention)
    return s.db.Update(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false
        it := txn.NewIterator(opts)
        defer it.Close()

        prefix := []byte("/deleted")
        for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
            item := it.Item()
            key := item.KeyCopy(nil)
            if item.ExpiresAt() != 0 && time.Unix(int64(item.ExpiresAt()), 0).Before(cutoff) {
                if err := txn.Delete(key); err != nil {
                    return err
                }
            }
        }
        return nil
    })
}
