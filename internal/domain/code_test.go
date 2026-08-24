package domain

import (
	"strings"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	for i := 0; i < 100; i++ {
		c, err := GenerateCode()
		if err != nil {
			t.Fatalf("GenerateCode: %v", err)
		}
		if len(c) != AutoCodeLength {
			t.Fatalf("length = %d, want %d", len(c), AutoCodeLength)
		}
		for _, ch := range c {
			if !strings.ContainsRune(codeAlphabet, ch) {
				t.Fatalf("character %q not in code alphabet", ch)
			}
		}
	}
}

func TestGenerateCodeUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		c, err := GenerateCode()
		if err != nil {
			t.Fatal(err)
		}
		if seen[c] {
			t.Fatalf("collision on %s", c)
		}
		seen[c] = true
	}
}

func TestGenerateCodeAlphabetExcludesConfusables(t *testing.T) {
	for _, ch := range "lo01" {
		if strings.ContainsRune(codeAlphabet, ch) {
			t.Fatalf("confusable character %q in alphabet", ch)
		}
	}
	for _, ch := range codeAlphabet {
		if ch >= 'A' && ch <= 'Z' {
			t.Fatalf("uppercase character %q in alphabet", ch)
		}
	}
}
