package domain

import (
	"strings"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	for _, n := range []int{4, 6, 8, 12} {
		for i := 0; i < 100; i++ {
			c, err := GenerateCode(n)
			if err != nil {
				t.Fatalf("GenerateCode(%d): %v", n, err)
			}
			if len(c) != n {
				t.Fatalf("length = %d, want %d", len(c), n)
			}
			for _, ch := range c {
				if !strings.ContainsRune(codeAlphabet, ch) {
					t.Fatalf("character %q not in code alphabet", ch)
				}
			}
		}
	}
}

func TestGenerateCodeUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		c, err := GenerateCode(DefaultCodeLength)
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

func TestValidateCodeLength(t *testing.T) {
	for _, n := range []int{4, 6, 12} {
		if err := ValidateCodeLength(n); err != nil {
			t.Fatalf("ValidateCodeLength(%d) = %v, want nil", n, err)
		}
	}
	for _, n := range []int{0, 1, 3, 13, 100} {
		if err := ValidateCodeLength(n); err == nil {
			t.Fatalf("ValidateCodeLength(%d) = nil, want error", n)
		}
	}
}
