package main

import (
	"net/http"
	"testing"

	"github.com/barats/shrl-io/internal/domain"
)

func TestSettingsEndpoints(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)
	_, aliceTok := newUser(t, s, "alice", false)

	// settings are admin-only
	if rec := do(t, s, "GET", "/settings", aliceTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin get settings = %d, want 403", rec.Code)
	}
	if rec := do(t, s, "PATCH", "/settings/code-length", aliceTok, map[string]any{"code_length": 8}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin set settings = %d, want 403", rec.Code)
	}

	// default is 6
	var out struct {
		CodeLength int `json:"code_length"`
	}
	decode(t, do(t, s, "GET", "/settings", adminTok, nil), &out)
	if out.CodeLength != domain.DefaultCodeLength {
		t.Fatalf("default code length = %d, want %d", out.CodeLength, domain.DefaultCodeLength)
	}

	// out-of-bounds values are rejected
	for _, bad := range []int{3, 13} {
		if rec := do(t, s, "PATCH", "/settings/code-length", adminTok, map[string]any{"code_length": bad}); rec.Code != http.StatusBadRequest {
			t.Fatalf("set %d = %d, want 400", bad, rec.Code)
		}
	}

	// a valid update persists
	if rec := do(t, s, "PATCH", "/settings/code-length", adminTok, map[string]any{"code_length": 8}); rec.Code != 200 {
		t.Fatalf("set 8 = %d, body %s", rec.Code, rec.Body.String())
	}
	decode(t, do(t, s, "GET", "/settings", adminTok, nil), &out)
	if out.CodeLength != 8 {
		t.Fatalf("code length after set = %d, want 8", out.CodeLength)
	}
}

func TestCodeLengthAffectsCreatedLinks(t *testing.T) {
	s := newTestServer(t)
	_, adminTok := newUser(t, s, "admin", true)

	for _, n := range []int{4, 8, 12} {
		if rec := do(t, s, "PATCH", "/settings/code-length", adminTok, map[string]any{"code_length": n}); rec.Code != 200 {
			t.Fatalf("set length %d = %d", n, rec.Code)
		}
		var link struct {
			Code string `json:"code"`
		}
		decode(t, do(t, s, "POST", "/links", adminTok, map[string]any{"destination": "https://example.com"}), &link)
		if len(link.Code) != n {
			t.Fatalf("created code length = %d, want %d", len(link.Code), n)
		}
	}
}
