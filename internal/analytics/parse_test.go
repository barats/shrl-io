package analytics

import "testing"

func TestReferrerHost(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "direct"},
		{"https://google.com/search", "google.com"},
		{"https://www.reddit.com/r/golang", "reddit.com"},
		{"http://news.ycombinator.com/item?id=1", "news.ycombinator.com"},
		{"not a url", "direct"},
	}
	for _, c := range cases {
		if got := ReferrerHost(c.in); got != c.want {
			t.Errorf("ReferrerHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClassifyUA(t *testing.T) {
	device, os, browser := ClassifyUA("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if device != "desktop" {
		t.Errorf("device = %q, want desktop", device)
	}
	if browser != "Chrome" {
		t.Errorf("browser = %q, want Chrome", browser)
	}
	if os == "" {
		t.Error("os is empty")
	}

	device, _, _ = ClassifyUA("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1")
	if device != "mobile" {
		t.Errorf("device = %q, want mobile", device)
	}
}

func TestVisitorHashStableAndPrivate(t *testing.T) {
	a := VisitorHash("1.2.3.4", "Chrome/120")
	b := VisitorHash("1.2.3.4", "chrome/120")
	c := VisitorHash("1.2.3.4", "Chrome/121")
	if a != b {
		t.Error("hash should be case-insensitive on user-agent")
	}
	if a == c {
		t.Error("different user-agents should hash differently")
	}
	if len(a) != 64 {
		t.Errorf("hash length = %d, want 64", len(a))
	}
}
