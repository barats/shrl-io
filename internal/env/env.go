package env

import (
	"os"
	"strconv"
	"time"
)

// Or returns the value of the environment variable key, or def when unset.
func Or(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Int returns the integer value of key, or def when unset or unparsable.
func Int(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// Duration returns the duration value of key, or def when unset or unparsable.
func Duration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
