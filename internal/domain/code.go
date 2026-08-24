package domain

import (
	"crypto/rand"
)

// AutoCodeLength is the length of auto-generated codes.
const AutoCodeLength = 6

// codeAlphabet is lowercase alphanumerics minus the visually confusable
// characters l, o, 0, and 1, so codes are unambiguous to hand-type. Users
// never choose a Code; shrl.io generates every one.
const codeAlphabet = "abcdefghijkmnpqrstuvwxyz23456789"

// GenerateCode returns a random lowercase code from the unambiguous alphabet.
func GenerateCode() (string, error) {
	b := make([]byte, AutoCodeLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b), nil
}
