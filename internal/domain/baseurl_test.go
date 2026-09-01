package domain

import "testing"

func TestNormalizeAndValidateBaseURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"example.com", "https://example.com"},
		{"https://example.com", "https://example.com"},
		{"http://example.com", "http://example.com"},
		{"https://example.com/", "https://example.com"},
		{"https://example.com/i", "https://example.com/i"},
		{"https://example.com/i/", "https://example.com/i"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"HTTP://Example.COM", "http://example.com"},
		{" https://example.com/path ", "https://example.com/path"},
	}
	for _, c := range cases {
		got, err := NormalizeAndValidateBaseURL(c.in)
		if err != nil {
			t.Errorf("NormalizeAndValidateBaseURL(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeAndValidateBaseURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeAndValidateBaseURLRejects(t *testing.T) {
	inputs := []string{
		"",
		"ftp://example.com",
		"https://example.com?x=1",
		"https://example.com#frag",
		"not a url",
	}
	for _, in := range inputs {
		if _, err := NormalizeAndValidateBaseURL(in); err == nil {
			t.Errorf("NormalizeAndValidateBaseURL(%q) expected error, got nil", in)
		}
	}
}
