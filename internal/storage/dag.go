package storage

import (
    "context"
    "crypto/sha256"
    "encoding/json"

    "github.com/Alyanaky/SecureDAG/internal/dag"
    "github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) StoreDAG(ctx context.Context, node *dag.MerkleNode) error {
    hash := sha256.Sum256(node.Hash)
    data, err := json.Marshal(node)
    if err != nil {
        return err
    }
    return s.db.Update(func(txn *badger.Txn) error {
        return txn.Set(hash[:], data)
    })
}
