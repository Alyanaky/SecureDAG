package crypto

import (
    "crypto/rand"
    "crypto/rsa"
    "sync"
    "time"
)

func RotateKeys(km *KeyManager, interval time.Duration) error {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for range ticker.C {
        km.mu.Lock()
        newPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
        if err != nil {
            km.mu.Unlock()
            return err
        }
        newPubKey := &newPrivKey.PublicKey
        km.SetKeys(newPrivKey, newPubKey)
        km.mu.Unlock()
    }
    return nil
}
