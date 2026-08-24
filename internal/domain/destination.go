package domain

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// NormalizeAndValidateDestination checks a destination URL and returns a
// normalized form. Only http/https are accepted, missing schemes default to
// https, and hosts that resolve to loopback/private/link-local addresses are
// rejected. Runs at creation/update time only, never on the redirect path.
func NormalizeAndValidateDestination(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("destination is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("destination is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("destination scheme must be http or https")
	}
	if u.Host == "" {
		return "", errors.New("destination must include a host")
	}
	if err := rejectPrivateHost(u.Hostname()); err != nil {
		return "", err
	}
	return u.String(), nil
}

func rejectPrivateHost(host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return errors.New("destination resolves to a blocked (private/loopback) address")
		}
		return nil
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		// Unresolvable hosts are allowed: creation-time checks are a snapshot,
		// and internal-only hostnames should remain usable.
		return nil
	}
	for _, ip := range addrs {
		if isBlockedIP(ip) {
			return errors.New("destination resolves to a blocked (private/loopback) address")
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}
