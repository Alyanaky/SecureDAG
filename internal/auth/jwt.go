package auth

import (
    "crypto/rand"
    "crypto/rsa"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

var (
    privateKey *rsa.PrivateKey
    publicKey  *rsa.PublicKey
)

// SetTempKeyForTesting generates a temporary RSA key pair for testing purposes.
func SetTempKeyForTesting(expiration time.Time) {
    var err error
    privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        panic(err)
    }
    publicKey = &privateKey.PublicKey
}

func GenerateToken(username string, customClaims map[string]interface{}) (string, error) {
    if privateKey == nil {
        return "", jwt.ErrInvalidKey
    }

    claims := jwt.MapClaims{
        "sub": username,
        "iat": jwt.NewNumericDate(time.Now()),
        "exp": jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
    }

    for k, v := range customClaims {
        claims[k] = v
    }

    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(privateKey)
}
