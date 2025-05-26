package crypto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"time"
)

const (
	KeyRotationInterval = 30 * 24 * time.Hour
)

func (m *KeyManager) StartRotation(ctx context.Context, reencryptFunc func(oldPriv *rsa.PrivateKey, newPub *rsa.PublicKey) error) {
	ticker := time.NewTicker(KeyRotationInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := m.RotateKeys(reencryptFunc); err != nil {
					// log error
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *KeyManager) RotateKeys(reencryptFunc func(oldPriv *rsa.PrivateKey, newPub *rsa.PublicKey) error) error {
	newPriv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}
	newPub := &newPriv.PublicKey
	m.mu.Lock()
	oldPriv := m.privateKey
	m.privateKey = newPriv
	m.publicKey = newPub
	m.mu.Unlock()
	if err := reencryptFunc(oldPriv, newPub); err != nil {
		return err
	}
	return nil
}
