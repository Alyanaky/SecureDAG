package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptData(t *testing.T) {
	km := NewKeyManager()
	data := []byte("test data")
	aesKey := make([]byte, 32)
	_, err := rand.Read(aesKey)
	require.NoError(t, err)

	// Encrypt data
	encrypted, err := km.EncryptData(data, aesKey)
	require.NoError(t, err)
	assert.NotEqual(t, data, encrypted)

	// Decrypt data
	decrypted, err := km.DecryptData(encrypted, aesKey)
	require.NoError(t, err)
	assert.Equal(t, data, decrypted)
}

func TestEncryptDecryptAESKey(t *testing.T) {
	km := NewKeyManager()
	aesKey := make([]byte, 32)
	_, err := rand.Read(aesKey)
	require.NoError(t, err)

	// Encrypt AES key
	encryptedKey, err := km.EncryptAESKey(km.GetPublicKey(), aesKey)
	require.NoError(t, err)
	assert.NotEqual(t, aesKey, encryptedKey)

	// Decrypt AES key
	decryptedKey, err := km.DecryptAESKey(km.GetPrivateKey(), encryptedKey)
	require.NoError(t, err)
	assert.Equal(t, aesKey, decryptedKey)
}

func TestKeyRotation(t *testing.T) {
	km := NewKeyManager()
	data := []byte("test data")
	aesKey := make([]byte, 32)
	_, err := rand.Read(aesKey)
	require.NoError(t, err)

	// Encrypt data with original key
	encryptedKey, err := km.EncryptAESKey(km.GetPublicKey(), aesKey)
	require.NoError(t, err)

	// Rotate keys
	oldPriv := km.GetPrivateKey()
	newPriv, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)
	newPub := &newPriv.PublicKey
	km.SetKeys(newPriv, newPub)

	// Decrypt with old key
	decryptedKey, err := km.DecryptAESKey(oldPriv, encryptedKey)
	require.NoError(t, err)
	assert.Equal(t, aesKey, decryptedKey)

	// Re-encrypt with new key
	newEncryptedKey, err := km.EncryptAESKey(newPub, decryptedKey)
	require.NoError(t, err)
	newDecryptedKey, err := km.DecryptAESKey(newPriv, newEncryptedKey)
	require.NoError(t, err)
	assert.Equal(t, aesKey, newDecryptedKey)
}
