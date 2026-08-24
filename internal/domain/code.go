package domain

import (
	"crypto/rand"
	"errors"
	"regexp"
)

const (
	// AutoCodeLength is the length of auto-generated codes (base62).
	AutoCodeLength = 6
	// maxCustomCodeLen bounds user-supplied codes.
	maxCustomCodeLen = 32
)

const base62Alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var customCodeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// GenerateCode returns a random case-sensitive base62 code.
func GenerateCode() (string, error) {
	b := make([]byte, AutoCodeLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = base62Alphabet[int(b[i])%len(base62Alphabet)]
	}
	return string(b), nil
}

// ValidateCustomCode checks a user-supplied code against the allowed alphabet
// and length. Codes are case-sensitive.
func ValidateCustomCode(code string) error {
	if !customCodeRe.MatchString(code) {
		return errors.New("code must be 1-32 characters of [A-Za-z0-9_-]")
	}
	return nil
}
