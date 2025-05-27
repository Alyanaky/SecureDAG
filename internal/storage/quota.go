package storage

import (
    "context"
    "encoding/binary"
    "errors"
    "sync"

    "github.com/dgraph-io/badger/v4"
)

type QuotaManager struct {
    store *BadgerStore
    mu    sync.Mutex
}

func NewQuotaManager(store *BadgerStore) *QuotaManager {
    return &QuotaManager{store: store}
}

func (q *QuotaManager) CheckQuota(ctx context.Context, bucket string, size int64) error {
    q.mu.Lock()
    defer q.mu.Unlock()

    quota, err := q.getQuota(bucket)
    if err != nil {
        return err
    }
    usage, err := q.getUsage(bucket)
    if err != nil {
        return err
    }

    if usage+size > quota {
        return errors.New("quota exceeded")
    }

    return q.setUsage(bucket, usage+size)
}

func (q *QuotaManager) getQuota(bucket string) (int64, error) {
    var quota int64
    err := q.store.db.View(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte("quota/" + bucket))
        if err != nil {
            return err
        }
        return item.Value(func(val []byte) error {
            quota = int64(binary.BigEndian.Uint64(val))
            return nil
        })
    })
    if err == badger.ErrKeyNotFound {
        return 1024 * 1024 * 1024, nil // Default 1GB
    }
    return quota, err
}

func (q *QuotaManager) getUsage(bucket string) (int64, error) {
    var usage int64
    err := q.store.db.View(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte("usage/" + bucket))
        if err != nil {
            return err
        }
        return item.Value(func(val []byte) error {
            usage = int64(binary.BigEndian.Uint64(val))
            return nil
        })
    })
    if err == badger.ErrKeyNotFound {
        return 0, nil
    }
    return usage, err
}

func (q *QuotaManager) setUsage(bucket string, usage int64) error {
    return q.store.db.Update(func(txn *badger.Txn) error {
        val := make([]byte, 8)
        binary.BigEndian.PutUint64(val, uint64(usage))
        return txn.Set([]byte("usage/" + bucket), val)
    })
}
