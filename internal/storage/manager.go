package storage

type StorageManager struct {
    store *BadgerStore
}

func NewStorageManager(store *BadgerStore) *StorageManager {
    return &StorageManager{store: store}
}
