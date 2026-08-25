package cache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/barats/shrl-io/internal/domain"
)

func testClient(t *testing.T) *redis.Client {
	t.Helper()
	rdb := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: rdb.Addr()})
}

func TestLinkCachePutGet(t *testing.T) {
	c := NewLinkCache(testClient(t))
	ctx := context.Background()
	l := &domain.Link{Hostname: "shrl.io", Code: "abc", Destination: "https://example.com/path?a=1", ForwardUTM: true}
	if err := c.Put(ctx, l); err != nil {
		t.Fatal(err)
	}
	got, ok, err := c.Get(ctx, "shrl.io", "abc")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.Destination != "https://example.com/path?a=1" {
		t.Errorf("destination = %q", got.Destination)
	}
	if !got.ForwardUTM {
		t.Error("forward_utm should round-trip through the cache")
	}
}

func TestLinkCacheDisabledEvicts(t *testing.T) {
	c := NewLinkCache(testClient(t))
	ctx := context.Background()
	if err := c.Put(ctx, &domain.Link{Hostname: "shrl.io", Code: "abc", Destination: "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Put(ctx, &domain.Link{Hostname: "shrl.io", Code: "abc", Destination: "https://example.com", Disabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := c.Get(ctx, "shrl.io", "abc"); err != nil || ok {
		t.Fatalf("disabled link should be evicted: ok=%v err=%v", ok, err)
	}
}
