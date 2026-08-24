package domain

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Hostname is a domain an Admin registers in the Hostname Registry, on which
// Links are served. Users may create Links under any registered Hostname.
type Hostname struct {
	Name         string    `json:"name" gorm:"primaryKey"`
	RegisteredBy int64     `json:"registered_by"`
	CreatedAt    time.Time `json:"created_at"`
}

var hostnameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

// NormalizeAndValidateHostname canonicalizes a hostname for the registry: a
// bare host, lowercased, with no scheme, port, path, query, or fragment.
// Scheme-prefixed input ("https://example.com") is accepted and reduced to
// the bare host.
func NormalizeAndValidateHostname(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "/")
	if raw == "" {
		return "", errors.New("hostname is required")
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return "", errors.New("hostname is not a valid host")
		}
		if (u.Scheme != "http" && u.Scheme != "https") ||
			u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return "", errors.New("hostname must be a bare host, no scheme/path/query/fragment")
		}
		raw = u.Host
	}
	if strings.Contains(raw, ":") {
		return "", errors.New("hostname must not include a port")
	}
	raw = strings.ToLower(raw)
	if len(raw) > 253 || !hostnameRe.MatchString(raw) {
		return "", errors.New("hostname is not valid")
	}
	return raw, nil
}
