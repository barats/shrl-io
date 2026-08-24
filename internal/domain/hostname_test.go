package domain

import "testing"

func TestNormalizeHostname(t *testing.T) {
	cases := []struct{ in, want string }{
		{"localhost", "localhost"},
		{"example.com", "example.com"},
		{"EXAMPLE.com", "example.com"},
		{"  example.com  ", "example.com"},
		{"example.com/", "example.com"},
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"https://www.Example.com", "www.example.com"},
	}
	for _, c := range cases {
		got, err := NormalizeAndValidateHostname(c.in)
		if err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRejectBadHostname(t *testing.T) {
	bad := []string{
		"",
		"example.com:8080",
		"https://example.com:8443",
		"example.com/path",
		"https://example.com/path",
		"https://example.com?x=1",
		"ftp://example.com",
		"example.com.",
		"-example.com",
		"example-.com",
		"exa mple.com",
		"under_score.example.com",
	}
	for _, in := range bad {
		if _, err := NormalizeAndValidateHostname(in); err == nil {
			t.Errorf("expected rejection of %q", in)
		}
	}
}
