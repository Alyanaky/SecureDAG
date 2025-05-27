package storage

import (
    "context"
    "log"
    "time"

    "github.com/Alyanaky/SecureDAG/internal/crypto"
    "github.com/Alyanaky/SecureDAG/internal/p2p"
    "github.com/dgraph-io/badger/v4"
    "github.com/ipfs/go-cid"
    "github.com/multiformats/go-multihash"
)

const (
    HealInterval = 5 * time.Minute
    MinReplicas  = 3
)

type BadgerStore struct {
    db          *badger.DB
    keyManager  *crypto.KeyManager
    dht         *p2p.DHTOperations
    healInterval time.Duration
}

func NewBadgerStore(dir string) (*BadgerStore, error) {
    opts := badger.DefaultOptions(dir)
    opts = opts.WithSyncWrites(true)
    opts = opts.WithCompression(options.ZSTD)

    db, err := badger.Open(opts)
    if err != nil {
        return nil, err
    }

    km := crypto.NewKeyManager()
    go crypto.RotateKeys(km, 24*time.Hour)

    store := &BadgerStore{
        db:          db,
        keyManager:  km,
        dht:         p2p.NewDHTOperations(nil), // Предполагается, что DHT инициализируется позже
        healInterval: HealInterval,
    }

    return store, nil
}

func (s *BadgerStore) Close() error {
    return s.db.Close()
}

func (s *BadgerStore) PutObject(bucket, key string, data []byte) error {
    aesKey := make([]byte, 32)
    if _, err := rand.Read(aesKey); err != nil {
        return err
    }

    encryptedData, err := s.keyManager.EncryptData(data, aesKey)
    if err != nil {
        return err
    }

    encryptedAESKey, err := s.keyManager.EncryptAESKey(s.keyManager.GetPublicKey(), aesKey)
    if err != nil {
        return err
    }

    return s.db.Update(func(txn *badger.Txn) error {
        objKey := []byte(bucket + "/" + key)
        if err := txn.Set(objKey, encryptedData); err != nil {
            return err
        }
        keyEncKey := []byte(bucket + "/" + key + "/key")
        return txn.Set(keyEncKey, encryptedAESKey)
    })
}

func (s *BadgerStore) GetObject(bucket, key string) ([]byte, error) {
    var encryptedData, encryptedAESKey []byte
    err := s.db.View(func(txn *badger.Txn) error {
        objKey := []byte(bucket + "/" + key)
        item, err := txn.Get(objKey)
        if err != nil {
            return err
        }
        encryptedData, err = item.ValueCopy(nil)
        if err != nil {
            return err
        }

        keyEncKey := []byte(bucket + "/" + key + "/key")
        item, err = txn.Get(keyEncKey)
        if err != nil {
            return err
        }
        encryptedAESKey, err = item.ValueCopy(nil)
        return err
    })
    if err != nil {
        return nil, err
    }

    aesKey, err := s.keyManager.DecryptAESKey(s.keyManager.GetPrivateKey(), encryptedAESKey)
    if err != nil {
        return nil, err
    }

    return s.keyManager.DecryptData(encryptedData, aesKey)
}

func (s *BadgerStore) DeleteObject(bucket, key string) error {
    return s.db.Update(func(txn *badger.Txn) error {
        objKey := []byte(bucket + "/" + key)
        keyEncKey := []byte(bucket + "/" + key + "/key")
        if err := txn.Delete(objKey); err != nil {
            return err
        }
        return txn.Delete(keyEncKey)
    })
}

func (s *BadgerStore) healBlock(ctx context.Context) error {
    // Заглушка для метода self-healing
    log.Println("Running self-healing process")
    return nil
}
