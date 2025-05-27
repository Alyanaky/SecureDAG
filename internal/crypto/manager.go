package crypto

   import (
       "crypto/aes"
       "crypto/cipher"
       "crypto/rand"
       "crypto/rsa"
       "crypto/sha256"
       "errors"
   )

   type KeyManager struct {
       privateKey *rsa.PrivateKey
       publicKey  *rsa.PublicKey
   }

   func NewKeyManager() *KeyManager {
       privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
       if err != nil {
           panic(err)
       }
       return &KeyManager{
           privateKey: privateKey,
           publicKey:  &privateKey.PublicKey,
       }
   }

   func (km *KeyManager) GetPublicKey() *rsa.PublicKey {
       return km.publicKey
   }

   func (km *KeyManager) GetPrivateKey() *rsa.PrivateKey {
       return km.privateKey
   }

   func (km *KeyManager) SetKeys(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) {
       km.privateKey = privateKey
       km.publicKey = publicKey
   }

   func (km *KeyManager) EncryptData(data, aesKey []byte) ([]byte, error) {
       block, err := aes.NewCipher(aesKey)
       if err != nil {
           return nil, err
       }
       gcm, err := cipher.NewGCM(block)
       if err != nil {
           return nil, err
       }
       nonce := make([]byte, gcm.NonceSize())
       if _, err := rand.Read(nonce); err != nil {
           return nil, err
       }
       return gcm.Seal(nonce, nonce, data, nil), nil
   }

   func (km *KeyManager) DecryptData(encrypted, aesKey []byte) ([]byte, error) {
       block, err := aes.NewCipher(aesKey)
       if err != nil {
           return nil, err
       }
       gcm, err := cipher.NewGCM(block)
       if err != nil {
           return nil, err
       }
       if len(encrypted) < gcm.NonceSize() {
           return nil, errors.New("encrypted data too short")
       }
       nonce, ciphertext := encrypted[:gcm.NonceSize()], encrypted[gcm.NonceSize():]
       return gcm.Open(nil, nonce, ciphertext, nil)
   }

   func (km *KeyManager) EncryptAESKey(pubKey *rsa.PublicKey, aesKey []byte) ([]byte, error) {
       return rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, aesKey, nil)
   }

   func (km *KeyManager) DecryptAESKey(privKey *rsa.PrivateKey, encryptedKey []byte) ([]byte, error) {
       return rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, encryptedKey, nil)
   }
