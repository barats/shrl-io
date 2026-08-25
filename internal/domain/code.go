package domain

import (
	"crypto/rand"
	"errors"
)

// Code length bounds for auto-generated codes (ADR 0013): an Admin sets the
// exact per-instance length, bounded so codes stay hand-typeable and the code
// space stays usable.
const (
	DefaultCodeLength = 6
	MinCodeLength     = 4
	MaxCodeLength     = 12
)

// codeAlphabet is lowercase alphanumerics minus the visually confusable
// characters l, o, 0, and 1, so codes are unambiguous to hand-type. Users
// never choose a Code; shrl.io generates every one.
const codeAlphabet = "abcdefghijkmnpqrstuvwxyz23456789"

// ValidateCodeLength reports whether n is a legal Code Length.
func ValidateCodeLength(n int) error {
	if n < MinCodeLength || n > MaxCodeLength {
		return errors.New("code length must be between 4 and 12 characters")
	}
	return nil
}

// GenerateCode returns a random lowercase code of length n from the
// unambiguous alphabet.
func GenerateCode(n int) (string, error) {
	if err := ValidateCodeLength(n); err != nil {
		return "", err
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b), nil
}
