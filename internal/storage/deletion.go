package storage

import (
    "context"
    "fmt"
    "sync"
    "time"
)

type DeletionManager struct {
    mu      sync.Mutex
    pending map[string]chan bool
}

func NewDeletionManager() *DeletionManager {
    return &DeletionManager{
        pending: make(map[string]chan bool),
    }
}

func (dm *DeletionManager) ScheduleDeletion(hash string, timeout time.Duration) error {
    dm.mu.Lock()
    defer dm.mu.Unlock()
    
    if _, exists := dm.pending[hash]; exists {
        return fmt.Errorf("deletion already scheduled")
    }
    
    done := make(chan bool, 1)
    dm.pending[hash] = done
    
    go func() {
        select {
        case <-time.After(timeout):
            dm.forceDelete(hash)
        case <-done:
            return
        }
    }()
    
    return nil
}

func (dm *DeletionManager) ConfirmDeletion(hash string) {
    dm.mu.Lock()
    defer dm.mu.Unlock()
    
    if done, exists := dm.pending[hash]; exists {
        close(done)
        delete(dm.pending, hash)
    }
}

func (dm *DeletionManager) forceDelete(hash string) {
    dm.mu.Lock()
    defer dm.mu.Unlock()
    
    // Логика удаления данных из хранилища
    store.DeleteBlock(hash)
    delete(dm.pending, hash)
}
