package helpers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateRandomString, kriptografik olarak güvenli bir kaynaktan n byte
// entropi üretir ve URL-safe base64 ile kodlanmış string olarak döner.
// Refresh token, API key gibi tahmin edilmesi imkansız olması gereken
// değerler için kullanılır.
func GenerateRandomString(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("invalid byte length: %d", n)
	}

	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
