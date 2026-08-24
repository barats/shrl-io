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
			if !strings.ContainsRune(base62Alphabet, ch) {
				t.Fatalf("character %q not in base62 alphabet", ch)
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

func TestValidateCustomCode(t *testing.T) {
	valid := []string{"abc", "ABC123", "a_b-c", "x", "A"}
	for _, c := range valid {
		if err := ValidateCustomCode(c); err != nil {
			t.Errorf("valid code %q rejected: %v", c, err)
		}
	}
	invalid := []string{"", "ab cd", "a/b", "a!b", "a.b", strings.Repeat("a", 33)}
	for _, c := range invalid {
		if err := ValidateCustomCode(c); err == nil {
			t.Errorf("invalid code %q accepted", c)
		}
	}
}
