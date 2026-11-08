package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewToken returns a random 32-byte URL-safe token together with its
// SHA-256 hash, which is the only thing stored in the database.
func NewToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken returns SHA-256 of the token as 32 raw bytes (BINARY(32)).
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
