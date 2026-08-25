package store

import (
	"context"
	"testing"

	"github.com/barats/shrl-io/internal/domain"
)

func TestSettingStoreCodeLength(t *testing.T) {
	db := newTestDB(t)
	s := NewSettingStore(db)
	ctx := context.Background()

	// unset -> default, and not present
	if n, err := s.CodeLength(ctx); err != nil || n != domain.DefaultCodeLength {
		t.Fatalf("CodeLength unset = %d, %v (want %d)", n, err, domain.DefaultCodeLength)
	}
	if ok, err := s.Has(ctx, domain.CodeLengthSetting); err != nil || ok {
		t.Fatalf("Has unset = %v, %v (want false)", ok, err)
	}

	// set and read back
	if err := s.SetCodeLength(ctx, 4); err != nil {
		t.Fatalf("set 4: %v", err)
	}
	if n, err := s.CodeLength(ctx); err != nil || n != 4 {
		t.Fatalf("CodeLength after set = %d, %v (want 4)", n, err)
	}
	if ok, err := s.Has(ctx, domain.CodeLengthSetting); err != nil || !ok {
		t.Fatalf("Has after set = %v, %v (want true)", ok, err)
	}

	// out-of-bounds values are rejected and leave the value unchanged
	for _, bad := range []int{3, 13} {
		if err := s.SetCodeLength(ctx, bad); err == nil {
			t.Fatalf("SetCodeLength(%d) = nil, want error", bad)
		}
	}
	if n, err := s.CodeLength(ctx); err != nil || n != 4 {
		t.Fatalf("CodeLength after bad set = %d, %v (want unchanged 4)", n, err)
	}
}
