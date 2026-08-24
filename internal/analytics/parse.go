package analytics

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"time"

	"github.com/mileusna/useragent"
)

// VisitorHash is the stable, privacy-safe identity for a visitor: a hash of
// IP + normalized user-agent. Raw IPs are never stored.
func VisitorHash(ip, userAgent string) string {
	h := sha256.Sum256([]byte(ip + "\x00" + strings.ToLower(strings.TrimSpace(userAgent))))
	return hex.EncodeToString(h[:])
}

// ReferrerHost extracts the normalized host from a referrer URL, or "direct".
func ReferrerHost(referrer string) string {
	if referrer == "" {
		return "direct"
	}
	u, err := url.Parse(referrer)
	if err != nil || u.Host == "" {
		return "direct"
	}
	host := strings.ToLower(u.Hostname())
	return strings.TrimPrefix(host, "www.")
}

// ClassifyUA returns (device, os, browser) for a user-agent string.
func ClassifyUA(ua string) (device, os, browser string) {
	p := useragent.Parse(ua)
	browser = p.Name
	os = p.OS
	switch {
	case p.Tablet:
		device = "tablet"
	case p.Mobile:
		device = "mobile"
	default:
		device = "desktop"
	}
	if browser == "" {
		browser = "unknown"
	}
	if os == "" {
		os = "unknown"
	}
	return
}

// DayOf returns the UTC date bucket for a visit timestamp. now is used when
// the timestamp is malformed.
func DayOf(ts string, now func() time.Time) time.Time {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t = now()
	}
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
