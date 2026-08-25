package analytics

import (
	"net/url"
	"strings"
)

// maxUTMLength is the cap on a recorded UTM parameter value. utm_term and
// utm_content are often per-visitor unique; truncation bounds Breakdown
// cardinality.
const maxUTMLength = 128

// UTMParams are the six standard UTM query parameters, in canonical order.
var UTMParams = []string{
	"utm_source",
	"utm_medium",
	"utm_campaign",
	"utm_term",
	"utm_content",
	"utm_id",
}

// UTMValues extracts the recognized UTM parameters from a short URL's query
// string. Absent parameters are empty strings.
func UTMValues(q url.Values) map[string]string {
	out := make(map[string]string, len(UTMParams))
	for _, p := range UTMParams {
		out[p] = q.Get(p)
	}
	return out
}

// NormalizeUTMValue trims and bounds a UTM parameter value for storage:
// empty becomes "unknown" (the same convention as every other dimension), and
// values longer than maxUTMLength are truncated.
func NormalizeUTMValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	r := []rune(v)
	if len(r) > maxUTMLength {
		return string(r[:maxUTMLength])
	}
	return v
}

// MergeUTMIntoDestination appends the recognized UTM parameters present in
// utm onto dest: an incoming value overrides a same-named parameter already
// on dest, dest's other query parameters are preserved, and empty values are
// skipped. Used by the redirector only when a Link's Forward UTM setting is
// on.
func MergeUTMIntoDestination(dest string, utm map[string]string) (string, error) {
	u, err := url.Parse(dest)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for _, p := range UTMParams {
		if v := strings.TrimSpace(utm[p]); v != "" {
			q.Set(p, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
