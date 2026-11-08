package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword hashes a plaintext password with bcrypt (DefaultCost).
// It rejects passwords longer than the 72-byte bcrypt input limit.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword reports whether the plaintext password matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
