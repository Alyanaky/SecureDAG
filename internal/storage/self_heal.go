package storage

import (
    "context"
    "time"

    "github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) SelfHeal(ctx context.Context) error {
    ticker := time.NewTicker(s.healInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            if err := s.healBlock(ctx); err != nil {
                return err
            }
        }
    }
}
