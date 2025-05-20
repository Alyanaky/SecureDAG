package crypto

import (
	"time"
)

const (
	KeyRotationInterval = 30 * 24 * time.Hour
)

func (m *KeyManager) StartRotation(ctx context.Context) {
	ticker := time.NewTicker(KeyRotationInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.RotateKeys()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *KeyManager) RotateKeys() {
	newPriv, _ := rsa.GenerateKey(rand.Reader, 4096)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.privateKey = newPriv
	m.publicKey = &newPriv.PublicKey
}
