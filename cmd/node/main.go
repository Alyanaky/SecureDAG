package main

import (
    "context"
    "log"

    "github.com/Alyanaky/SecureDAG/internal/storage"
)

func main() {
    store, err := storage.NewBadgerStore("/tmp/securedag")
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    ctx := context.Background()
    go func() {
        if err := store.SelfHeal(ctx); err != nil {
            log.Printf("Self-healing failed: %v", err)
        }
    }()

    select {}
}
