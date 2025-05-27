package crypto

import (
    "crypto/rand"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestKeyManager_EncryptData(t *testing.T) {
    km := NewKeyManager()
    aesKey := make([]byte, 32)
    _, err := rand.Read(aesKey)
    require.NoError(t, err)

    originalData := []byte("sensitive data")
    encrypted, err := km.EncryptData(originalData, aesKey)
    require.NoError(t, err)

    decrypted, err := km.DecryptData(encrypted, aesKey)
    require.NoError(t, err)
    assert.Equal(t, originalData, decrypted)
}
