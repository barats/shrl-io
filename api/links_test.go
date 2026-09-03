package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/barats/shrl-io/internal/store"
)

// The Links list carries each link's all-time visit total: a link with
// recorded visits shows its lifetime count, one with none shows zero.
func TestLinkListIncludesLifetimeVisits(t *testing.T) {
	s := newTestServer(t)
	_, tok := newUser(t, s, "alice", false)

	rec := do(t, s, "POST", "/links", tok, map[string]any{"destination": "https://visited.example.com"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create visited link = %d, body %s", rec.Code, rec.Body.String())
	}
	var visited struct {
		Code string `json:"code"`
	}
	decode(t, rec, &visited)

	rec = do(t, s, "POST", "/links", tok, map[string]any{"destination": "https://unvisited.example.com"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create unvisited link = %d, body %s", rec.Code, rec.Body.String())
	}

	if err := s.analytics.ApplyAnalytics(context.Background(), nil,
		[]store.LifetimeIncrement{{Code: visited.Code, Visits: 7}}, nil); err != nil {
		t.Fatalf("seed lifetime: %v", err)
	}

	rec = do(t, s, "GET", "/links", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list links = %d (body %s)", rec.Code, rec.Body.String())
	}
	var items []map[string]any
	decode(t, rec, &items)
	if len(items) != 2 {
		t.Fatalf("links = %d items, want 2", len(items))
	}
	byCode := map[string]float64{}
	for _, it := range items {
		code, _ := it["code"].(string)
		v, ok := it["visits"].(float64)
		if !ok {
			t.Fatalf("link %s has no visits field: %v", code, it)
		}
		byCode[code] = v
	}
	if byCode[visited.Code] != 7 {
		t.Fatalf("visited link = %v, want 7", byCode[visited.Code])
	}
	for code, v := range byCode {
		if code != visited.Code && v != 0 {
			t.Fatalf("unvisited link %s = %v, want 0", code, v)
		}
	}
}
