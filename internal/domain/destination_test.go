package domain

import "testing"

func TestNormalizeDestination(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://example.com", "https://example.com"},
		{"example.com/path", "https://example.com/path"},
		{"http://example.com", "http://example.com"},
		{"  https://example.com/x  ", "https://example.com/x"},
	}
	for _, c := range cases {
		got, err := NormalizeAndValidateDestination(c.in)
		if err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRejectBadDestination(t *testing.T) {
	bad := []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"data:text/html;base64,xxx",
		"ftp://example.com",
		"http://127.0.0.1/x",
		"http://localhost/x",
		"http://10.0.0.1/x",
		"http://192.168.1.1/x",
		"http://[::1]/x",
	}
	for _, in := range bad {
		if _, err := NormalizeAndValidateDestination(in); err == nil {
			t.Errorf("expected rejection of %q", in)
		}
	}
}
