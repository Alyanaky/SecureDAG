package sync

import (
    "context"
    "log"

    "github.com/Alyanaky/SecureDAG/internal/storage"
)

type GossipSync struct {
    storage *storage.BadgerStore
}

func NewGossipSync(storage *storage.BadgerStore) *GossipSync {
    return &GossipSync{storage: storage}
}

func (g *GossipSync) Sync(ctx context.Context, key string, data []byte) error {
    // Заглушка для синхронизации через gossip
    log.Printf("Syncing key %s", key)
    return g.storage.PutObject("sync", key, data)
}
