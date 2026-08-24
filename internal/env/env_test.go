package env

import (
	"testing"
	"time"
)

func TestOr(t *testing.T) {
	t.Setenv("SHRL_TEST_OR", "set")
	if got := Or("SHRL_TEST_OR", "def"); got != "set" {
		t.Fatalf("Or(set) = %q, want %q", got, "set")
	}
	if got := Or("SHRL_TEST_UNSET_OR", "def"); got != "def" {
		t.Fatalf("Or(unset) = %q, want %q", got, "def")
	}
}

func TestInt(t *testing.T) {
	t.Setenv("SHRL_TEST_INT", "42")
	if got := Int("SHRL_TEST_INT", 0); got != 42 {
		t.Fatalf("Int(set) = %d, want 42", got)
	}
	t.Setenv("SHRL_TEST_BAD_INT", "abc")
	if got := Int("SHRL_TEST_BAD_INT", 7); got != 7 {
		t.Fatalf("Int(bad) = %d, want default 7", got)
	}
	if got := Int("SHRL_TEST_UNSET_INT", 7); got != 7 {
		t.Fatalf("Int(unset) = %d, want default 7", got)
	}
}

func TestDuration(t *testing.T) {
	t.Setenv("SHRL_TEST_DUR", "30s")
	if got := Duration("SHRL_TEST_DUR", time.Minute); got != 30*time.Second {
		t.Fatalf("Duration(set) = %v, want 30s", got)
	}
	t.Setenv("SHRL_TEST_BAD_DUR", "soon")
	if got := Duration("SHRL_TEST_BAD_DUR", time.Minute); got != time.Minute {
		t.Fatalf("Duration(bad) = %v, want default 1m", got)
	}
	if got := Duration("SHRL_TEST_UNSET_DUR", time.Minute); got != time.Minute {
		t.Fatalf("Duration(unset) = %v, want default 1m", got)
	}
}
