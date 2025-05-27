package storage

import (
    "context"
    "log"

    "github.com/Alyanaky/SecureDAG/internal/p2p"
    "github.com/dgraph-io/badger/v4"
)

type Replicator struct {
    store *BadgerStore
    dht   *p2p.DHTOperations
}

func NewReplicator(store *BadgerStore, dht *p2p.DHTOperations) *Replicator {
    return &Replicator{store: store, dht: dht}
}

func (r *Replicator) Replicate(ctx context.Context, bucket, key string) error {
    data, err := r.store.GetObject(bucket, key)
    if err != nil {
        return err
    }

    return r.dht.ReplicateData(ctx, bucket+"/"+key, data)
}

func (r *Replicator) EnsureReplicas(ctx context.Context) error {
    return r.store.db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = true
        it := txn.NewIterator(opts)
        defer it.Close()

        for it.Rewind(); it.Valid(); it.Next() {
            item := it.Item()
            key := item.KeyCopy(nil)
            if err := r.dht.ReplicateData(ctx, string(key), item.ValueCopy(nil)); err != nil {
                log.Printf("Failed to replicate key %s: %v", key, err)
            }
        }
        return nil
    })
}
