package analytics

import (
	"net/url"
	"testing"
)

func TestUTMValues(t *testing.T) {
	q := url.Values{}
	q.Set("utm_source", "newsletter")
	q.Set("utm_campaign", "spring-launch")
	q.Set("ref", "ignored")
	got := UTMValues(q)
	if got["utm_source"] != "newsletter" {
		t.Errorf("utm_source = %q, want newsletter", got["utm_source"])
	}
	if got["utm_campaign"] != "spring-launch" {
		t.Errorf("utm_campaign = %q, want spring-launch", got["utm_campaign"])
	}
	if got["utm_medium"] != "" {
		t.Errorf("utm_medium = %q, want empty", got["utm_medium"])
	}
	if len(got) != len(UTMParams) {
		t.Errorf("UTMValues returned %d params, want %d", len(got), len(UTMParams))
	}
}

func TestNormalizeUTMValue(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "unknown"},
		{"   ", "unknown"},
		{"newsletter", "newsletter"},
		{"  newsletter  ", "newsletter"},
	}
	for _, c := range cases {
		if got := NormalizeUTMValue(c.in); got != c.want {
			t.Errorf("NormalizeUTMValue(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// Multibyte runes: truncation must not split a UTF-8 sequence.
	long := ""
	for i := 0; i < maxUTMLength+10; i++ {
		long += "é"
	}
	got := NormalizeUTMValue(long)
	if len([]rune(got)) != maxUTMLength {
		t.Errorf("NormalizeUTMValue(long) rune length = %d, want %d", len([]rune(got)), maxUTMLength)
	}
}

func TestMergeUTMIntoDestination(t *testing.T) {
	utm := map[string]string{
		"utm_source":   "newsletter",
		"utm_campaign": "spring",
		"utm_medium":   " ",
	}
	cases := []struct {
		dest string
		want string
	}{
		{"https://example.com", "https://example.com?utm_campaign=spring&utm_source=newsletter"},
		{"https://example.com/path?a=1&utm_source=old", "https://example.com/path?a=1&utm_campaign=spring&utm_source=newsletter"},
	}
	for _, c := range cases {
		got, err := MergeUTMIntoDestination(c.dest, utm)
		if err != nil {
			t.Fatalf("MergeUTMIntoDestination(%q): %v", c.dest, err)
		}
		if got != c.want {
			t.Errorf("MergeUTMIntoDestination(%q) = %q, want %q", c.dest, got, c.want)
		}
	}
	// An empty incoming value is skipped entirely.
	got, err := MergeUTMIntoDestination("https://example.com", map[string]string{"utm_medium": "  "})
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com" {
		t.Errorf("empty utm_medium should be skipped, got %q", got)
	}
}
