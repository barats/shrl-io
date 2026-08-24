package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeRemark(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"   ", ""},
		{"  promo campaign  ", "promo campaign"},
		{strings.Repeat("a", MaxRemarkLength), strings.Repeat("a", MaxRemarkLength)},
		{strings.Repeat("中", 200), strings.Repeat("中", 200)}, // runes, not bytes
	}
	for _, c := range cases {
		got, err := NormalizeRemark(c.in)
		if err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRejectLongRemark(t *testing.T) {
	bad := []string{
		strings.Repeat("a", MaxRemarkLength+1),
		strings.Repeat("中", 201),
	}
	for _, in := range bad {
		if _, err := NormalizeRemark(in); err == nil {
			t.Errorf("expected rejection of remark of %d runes", utf8.RuneCountInString(in))
		}
	}
}
