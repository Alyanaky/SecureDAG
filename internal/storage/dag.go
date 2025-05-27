package storage

import (
    "context"
    "crypto/sha256"

    "github.com/Alyanaky/SecureDAG/internal/dag"
    "github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) StoreDAG(ctx context.Context, node *dag.MerkleNode) error {
    hash := sha256.Sum256(node.Hash)
    return s.db.Update(func(txn *badger.Txn) error {
        return txn.Set(hash[:], node.Marshal())
    })
}
