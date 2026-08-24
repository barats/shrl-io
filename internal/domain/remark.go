package domain

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// MaxRemarkLength bounds the optional Remark a Creator writes on a Link.
const MaxRemarkLength = 200

// NormalizeRemark trims a remark and rejects it when it exceeds the length
// cap. An empty remark is valid (Remark is optional).
func NormalizeRemark(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if utf8.RuneCountInString(raw) > MaxRemarkLength {
		return "", errors.New("remark must be 200 characters or fewer")
	}
	return raw, nil
}
