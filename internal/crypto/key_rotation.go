package crypto

import (
    "crypto/rand"
    "crypto/rsa"
    "time"
)

func RotateKeys(km *KeyManager, interval time.Duration) error {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for range ticker.C {
        newPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
        if err != nil {
            return err
        }
        newPubKey := &newPrivKey.PublicKey
        km.SetKeys(newPrivKey, newPubKey)
    }
    return nil
}
