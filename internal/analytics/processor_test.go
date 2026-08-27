package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/barats/shrl-io/internal/geo"
	"github.com/barats/shrl-io/internal/store"
)

type fakeGeo struct {
	loc geo.Location
}

func (f *fakeGeo) Lookup(ip string) geo.Location { return f.loc }

type fakeCache struct {
	seen map[string]bool
}

func (f *fakeCache) AddUniqueVisitor(_ context.Context, code string, day time.Time, hash string) (bool, error) {
	key := code + "|" + day.Format("2006-01-02") + "|" + hash
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	if f.seen[key] {
		return false, nil
	}
	f.seen[key] = true
	return true, nil
}

func (f *fakeCache) RemoveUniqueVisitor(_ context.Context, code string, day time.Time, hash string) error {
	key := code + "|" + day.Format("2006-01-02") + "|" + hash
	delete(f.seen, key)
	return nil
}

func (f *fakeCache) AddUniqueVisitorDim(_ context.Context, code string, day time.Time, dimension, value, hash string) (bool, error) {
	key := code + "|" + day.Format("2006-01-02") + "|" + dimension + "|" + value + "|" + hash
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	if f.seen[key] {
		return false, nil
	}
	f.seen[key] = true
	return true, nil
}

func (f *fakeCache) RemoveUniqueVisitorDim(_ context.Context, code string, day time.Time, dimension, value, hash string) error {
	key := code + "|" + day.Format("2006-01-02") + "|" + dimension + "|" + value + "|" + hash
	delete(f.seen, key)
	return nil
}

type fakeStore struct {
	dailies    []store.DailyIncrement
	lifetimes  []store.LifetimeIncrement
	breakdowns []store.BreakdownIncrement
	fail       bool
}

func (f *fakeStore) ApplyAnalytics(_ context.Context, d []store.DailyIncrement, l []store.LifetimeIncrement, b []store.BreakdownIncrement) error {
	if f.fail {
		return errors.New("apply failed")
	}
	f.dailies, f.lifetimes, f.breakdowns = d, l, b
	return nil
}

func chromeUA() string {
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
}

func TestProcessMessages(t *testing.T) {
	fs := &fakeStore{}
	p := &Processor{
		Cache: &fakeCache{},
		Store: fs,
		Now:   func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) },
	}
	msgs := []redis.XMessage{
		// human, google referrer
		{ID: "1-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "1.1.1.1",
			"user_agent": chromeUA(), "referrer": "https://google.com/search",
			"ts": "2026-08-24T10:00:00Z",
		}},
		// same visitor again -> visit but not a new unique
		{ID: "2-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "1.1.1.1",
			"user_agent": chromeUA(), "referrer": "https://google.com/search",
			"ts": "2026-08-24T11:00:00Z",
		}},
		// new visitor, no referrer
		{ID: "3-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "2.2.2.2",
			"user_agent": chromeUA(), "referrer": "",
			"ts": "2026-08-24T11:30:00Z",
		}},
		// bot -> excluded entirely
		{ID: "4-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "9.9.9.9",
			"user_agent": "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
			"referrer":   "", "ts": "2026-08-24T11:45:00Z",
		}},
		// missing code -> skipped
		{ID: "5-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "ip": "3.3.3.3", "user_agent": chromeUA(),
			"referrer": "", "ts": "2026-08-24T11:50:00Z",
		}},
	}

	if err := p.ProcessMessages(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}

	if len(fs.dailies) != 1 {
		t.Fatalf("dailies = %d, want 1", len(fs.dailies))
	}
	d := fs.dailies[0]
	if d.Visits != 3 {
		t.Errorf("visits = %d, want 3 (bot + missing-code excluded)", d.Visits)
	}
	if d.Uniques != 2 {
		t.Errorf("uniques = %d, want 2", d.Uniques)
	}
	if d.Day != time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC) {
		t.Errorf("day = %v, want 2026-08-24 UTC", d.Day)
	}

	if len(fs.lifetimes) != 1 {
		t.Fatalf("lifetimes = %d, want 1", len(fs.lifetimes))
	}
	if fs.lifetimes[0].Visits != 3 {
		t.Errorf("lifetime = %d, want 3", fs.lifetimes[0].Visits)
	}

	referrers := map[string]int64{}
	for _, b := range fs.breakdowns {
		if b.Dimension == "referrer" {
			referrers[b.Value] += b.Count
		}
	}
	if referrers["google.com"] != 2 || referrers["direct"] != 1 {
		t.Errorf("referrer breakdowns = %v, want google.com=2 direct=1", referrers)
	}

	// every human visit produced a device/os/browser row
	dims := map[string]bool{}
	for _, b := range fs.breakdowns {
		dims[b.Dimension] = true
	}
	for _, want := range []string{"referrer", "device", "os", "browser"} {
		if !dims[want] {
			t.Errorf("missing breakdown dimension %q", want)
		}
	}
}

// TestApplyFailureUndoesDedup verifies that when the DB apply fails, the
// batch's dedup additions are removed so a redelivered batch counts the same
// visitors as new again instead of permanently losing unique counts.
func TestProcessMessagesUTMDimensions(t *testing.T) {
	fs := &fakeStore{}
	p := &Processor{
		Cache: &fakeCache{},
		Store: fs,
		Now:   func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) },
	}
	msgs := []redis.XMessage{
		// carries utm_source and utm_campaign
		{ID: "1-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "1.1.1.1",
			"user_agent": chromeUA(), "referrer": "",
			"utm_source": "newsletter", "utm_campaign": "spring-launch",
			"ts": "2026-08-24T10:00:00Z",
		}},
		// no utm at all -> every dimension buckets "unknown"
		{ID: "2-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "2.2.2.2",
			"user_agent": chromeUA(), "referrer": "",
			"ts": "2026-08-24T11:00:00Z",
		}},
	}
	if err := p.ProcessMessages(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	byDim := map[string]map[string]int64{}
	for _, b := range fs.breakdowns {
		if _, ok := byDim[b.Dimension]; !ok {
			byDim[b.Dimension] = map[string]int64{}
		}
		byDim[b.Dimension][b.Value] += b.Count
	}
	if got := byDim["utm_source"]["newsletter"]; got != 1 {
		t.Errorf("utm_source=newsletter count = %d, want 1", got)
	}
	if got := byDim["utm_campaign"]["spring-launch"]; got != 1 {
		t.Errorf("utm_campaign=spring-launch count = %d, want 1", got)
	}
	if got := byDim["utm_source"]["unknown"]; got != 1 {
		t.Errorf("utm_source=unknown count = %d, want 1", got)
	}
	// every dimension is produced for the two human visits
	for _, dim := range UTMParams {
		var total int64
		for _, v := range byDim[dim] {
			total += v
		}
		if total != 2 {
			t.Errorf("dimension %s total = %d, want 2", dim, total)
		}
	}
}

func TestApplyFailureUndoesDedup(t *testing.T) {
	fc := &fakeCache{}
	fs := &fakeStore{fail: true}
	p := &Processor{
		Cache: fc,
		Store: fs,
		Now:   func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) },
	}
	msgs := []redis.XMessage{
		{ID: "1-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "1.1.1.1",
			"user_agent": chromeUA(), "referrer": "https://google.com",
			"ts": "2026-08-24T10:00:00Z",
		}},
	}

	if err := p.ProcessMessages(context.Background(), msgs); err == nil {
		t.Fatal("expected apply error")
	}
	if len(fc.seen) != 0 {
		t.Errorf("dedup set not restored after failed apply: %d entries remain", len(fc.seen))
	}

	// A retry (redelivery) must count the visitor as new again.
	fs.fail = false
	if err := p.ProcessMessages(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	if len(fs.dailies) != 1 || fs.dailies[0].Uniques != 1 {
		t.Errorf("after retry, uniques = %v, want 1", fs.dailies)
	}
}

func breakdownValue(t *testing.T, bs []store.BreakdownIncrement, dim string) int64 {
	t.Helper()
	var n int64
	for _, b := range bs {
		if b.Dimension == dim {
			n += b.Count
		}
	}
	return n
}

func TestProcessMessagesLocationAttribution(t *testing.T) {
	fs := &fakeStore{}
	p := &Processor{
		Cache: &fakeCache{},
		Store: fs,
		Geo:   &fakeGeo{loc: geo.Location{Country: "US", Region: "California", City: "San Francisco"}},
		Now:   func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) },
	}
	msgs := []redis.XMessage{
		{ID: "1-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "8.8.8.8",
			"user_agent": chromeUA(), "referrer": "https://google.com",
			"ts": "2026-08-24T10:00:00Z",
		}},
	}
	if err := p.ProcessMessages(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	if got := breakdownValue(t, fs.breakdowns, "country"); got != 1 {
		t.Errorf("country breakdown count = %d, want 1", got)
	}
	byValue := map[string]string{}
	for _, b := range fs.breakdowns {
		if b.Count > 0 {
			byValue[b.Dimension+"="+b.Value] = ""
		}
	}
	if _, ok := byValue["country=US"]; !ok {
		t.Errorf("missing country=US breakdown")
	}
	if _, ok := byValue["region=California"]; !ok {
		t.Errorf("missing region=California breakdown")
	}
	if _, ok := byValue["city=San Francisco"]; !ok {
		t.Errorf("missing city=San Francisco breakdown")
	}
}

func TestProcessMessagesLocationUnknownWithoutGeo(t *testing.T) {
	fs := &fakeStore{}
	p := &Processor{
		Cache: &fakeCache{},
		Store: fs,
		Now:   func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) },
	}
	msgs := []redis.XMessage{
		{ID: "1-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "10.0.0.5",
			"user_agent": chromeUA(), "referrer": "",
			"ts": "2026-08-24T10:00:00Z",
		}},
	}
	if err := p.ProcessMessages(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}
	for _, dim := range []string{"country", "region", "city"} {
		found := false
		for _, b := range fs.breakdowns {
			if b.Dimension == dim && b.Value == "unknown" {
				found = true
			}
		}
		if !found {
			t.Errorf("dimension %q should be 'unknown' without a geo resolver", dim)
		}
	}
}

// TestProcessMessagesDimensionUniques verifies per-dimension unique-visitor
// tracking: a deterministic dimension (browser) counts each visitor once, and
// a referrer the visitor appears under multiple values counts them per value.
func TestProcessMessagesDimensionUniques(t *testing.T) {
	fs := &fakeStore{}
	p := &Processor{
		Cache: &fakeCache{},
		Store: fs,
		Now:   func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) },
	}
	msgs := []redis.XMessage{
		// visitor A via google, twice
		{ID: "1-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "1.1.1.1",
			"user_agent": chromeUA(), "referrer": "https://google.com",
			"ts": "2026-08-24T10:00:00Z",
		}},
		{ID: "2-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "1.1.1.1",
			"user_agent": chromeUA(), "referrer": "https://google.com",
			"ts": "2026-08-24T11:00:00Z",
		}},
		// visitor B (same browser, different device identity), direct
		{ID: "3-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "2.2.2.2",
			"user_agent": chromeUA(), "referrer": "",
			"ts": "2026-08-24T11:30:00Z",
		}},
		// visitor A again, now from twitter
		{ID: "4-0", Values: map[string]interface{}{
			"hostname": "shrl.io", "code": "abc", "ip": "1.1.1.1",
			"user_agent": chromeUA(), "referrer": "https://twitter.com",
			"ts": "2026-08-24T11:45:00Z",
		}},
	}
	if err := p.ProcessMessages(context.Background(), msgs); err != nil {
		t.Fatal(err)
	}

	byDim := map[string]map[string]store.BreakdownIncrement{}
	for _, b := range fs.breakdowns {
		if byDim[b.Dimension] == nil {
			byDim[b.Dimension] = map[string]store.BreakdownIncrement{}
		}
		prev := byDim[b.Dimension][b.Value]
		prev.Count += b.Count
		prev.Uniques += b.Uniques
		byDim[b.Dimension][b.Value] = prev
	}

	// deterministic dimension: both visitors share one browser value.
	if len(byDim["browser"]) != 1 {
		t.Fatalf("browser values = %v, want 1 (both visitors share a browser)", byDim["browser"])
	}
	for value, inc := range byDim["browser"] {
		if inc.Count != 4 || inc.Uniques != 2 {
			t.Errorf("browser=%s count=%d uniques=%d, want 4 visits / 2 uniques", value, inc.Count, inc.Uniques)
		}
	}

	// referrer is per-value: visitor A counts once under google and once under
	// twitter; visitor B once under direct.
	referrer := byDim["referrer"]
	if referrer["google.com"].Count != 2 || referrer["google.com"].Uniques != 1 {
		t.Errorf("google.com = %+v, want count 2 uniques 1", referrer["google.com"])
	}
	if referrer["twitter.com"].Count != 1 || referrer["twitter.com"].Uniques != 1 {
		t.Errorf("twitter.com = %+v, want count 1 uniques 1", referrer["twitter.com"])
	}
	if referrer["direct"].Count != 1 || referrer["direct"].Uniques != 1 {
		t.Errorf("direct = %+v, want count 1 uniques 1", referrer["direct"])
	}
}
