package domain

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

// BaseURL is the public URL prefix under which Links are served (e.g.
// https://example.com or http://localhost:8080), registered by an Admin in
// the Registry. Users select a BaseURL when creating a Link; display and QR
// codes render {base_url}/{code}.
type BaseURL struct {
	BaseURL      string    `json:"base_url" gorm:"primaryKey"`
	RegisteredBy int64     `json:"registered_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// NormalizeAndValidateBaseURL canonicalizes a base URL for the registry: a
// scheme (http or https, https when omitted), a bare host with an optional
// port, and an optional path prefix, lowercased, with no query or fragment.
func NormalizeAndValidateBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("base URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("base URL is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("base URL scheme must be http or https")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("base URL must not include a query or fragment")
	}
	host := strings.ToLower(u.Host)
	path := strings.TrimRight(u.Path, "/")
	return u.Scheme + "://" + host + path, nil
}
