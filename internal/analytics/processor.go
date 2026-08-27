package analytics

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/geo"
	"github.com/barats/shrl-io/internal/store"
)

// Cache and Store are the seams the processor needs; satisfied by
// *cache.AnalyticsCache and *store.AnalyticsStore, faked in tests.
type Cache interface {
	AddUniqueVisitor(ctx context.Context, code string, day time.Time, hash string) (bool, error)
	RemoveUniqueVisitor(ctx context.Context, code string, day time.Time, hash string) error
	AddUniqueVisitorDim(ctx context.Context, code string, day time.Time, dimension, value, hash string) (bool, error)
	RemoveUniqueVisitorDim(ctx context.Context, code string, day time.Time, dimension, value, hash string) error
}

type Store interface {
	ApplyAnalytics(ctx context.Context, dailies []store.DailyIncrement, lifetimes []store.LifetimeIncrement, breakdowns []store.BreakdownIncrement) error
}

// GeoResolver attributes a visitor's IP to a location. Satisfied by
// *geo.Resolver; a nil Geo on the Processor disables location attribution
// (all locations become "unknown").
type GeoResolver interface {
	Lookup(ip string) geo.Location
}

// Processor turns batches of stream events into analytics deltas and applies
// them through the Store.
type Processor struct {
	Cache Cache
	Store Store
	Geo   GeoResolver
	// Now is injectable for deterministic tests; defaults to time.Now.
	Now func() time.Time
}

func (p *Processor) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

type dayLink struct {
	day  time.Time
	code string
}

type linkKey struct {
	code string
}

type breakdownKey struct {
	day       time.Time
	code      string
	dimension string
	value     string
}

type dimValue struct {
	dim, value string
}

// ProcessMessages turns raw stream events into analytics deltas and applies
// them in one transaction. Bots are skipped; unique visitors are deduplicated
// through the Cache, once per link-day and once per link-day-dimension-value.
func (p *Processor) ProcessMessages(ctx context.Context, msgs []redis.XMessage) error {
	dailies := map[dayLink]store.DailyIncrement{}
	lifetimes := map[linkKey]store.LifetimeIncrement{}
	breakdowns := map[breakdownKey]store.BreakdownIncrement{}
	var addedHashes []visitorHash
	var addedDimHashes []visitorDimHash

	for _, m := range msgs {
		code := strVal(m.Values, "code")
		ip := strVal(m.Values, "ip")
		ua := strVal(m.Values, "user_agent")
		referrer := strVal(m.Values, "referrer")
		ts := strVal(m.Values, "ts")
		if code == "" {
			continue
		}
		if domain.IsBot(ua) {
			continue
		}

		day := DayOf(ts, p.now)
		dl := dayLink{day: day, code: code}

		d := dailies[dl]
		d.Code, d.Day = code, day
		d.Visits++
		dailies[dl] = d

		hash := VisitorHash(ip, ua)
		added, err := p.Cache.AddUniqueVisitor(ctx, code, day, hash)
		if err != nil {
			return err
		}
		if added {
			d = dailies[dl]
			d.Uniques++
			dailies[dl] = d
			addedHashes = append(addedHashes, visitorHash{code, day, hash})
		}

		lk := linkKey{code: code}
		l := lifetimes[lk]
		l.Code = code
		l.Visits++
		lifetimes[lk] = l

		device, os, browser := ClassifyUA(ua)
		country, region, city := locationField(p.Geo, ip)
		dims := []dimValue{
			{"referrer", ReferrerHost(referrer)},
			{"device", device},
			{"os", os},
			{"browser", browser},
			{"country", country},
			{"region", region},
			{"city", city},
		}
		for _, dim := range UTMParams {
			dims = append(dims, dimValue{dim, NormalizeUTMValue(strVal(m.Values, dim))})
		}
		for _, dv := range dims {
			bk := breakdownKey{day: day, code: code, dimension: dv.dim, value: dv.value}
			b := breakdowns[bk]
			b.Code, b.Day, b.Dimension, b.Value = code, day, dv.dim, dv.value
			b.Count++
			added, err := p.Cache.AddUniqueVisitorDim(ctx, code, day, dv.dim, dv.value, hash)
			if err != nil {
				return err
			}
			if added {
				b.Uniques++
				addedDimHashes = append(addedDimHashes, visitorDimHash{code, day, dv.dim, dv.value, hash})
			}
			breakdowns[bk] = b
		}
	}

	if err := p.Store.ApplyAnalytics(ctx, values(dailies), values(lifetimes), values(breakdowns)); err != nil {
		// Undo this batch's dedup additions so a redelivery counts the same
		// visitors as new again; otherwise a failed batch would permanently
		// lose unique counts.
		for _, h := range addedHashes {
			p.Cache.RemoveUniqueVisitor(ctx, h.code, h.day, h.hash)
		}
		for _, h := range addedDimHashes {
			p.Cache.RemoveUniqueVisitorDim(ctx, h.code, h.day, h.dim, h.value, h.hash)
		}
		return err
	}
	return nil
}

type visitorHash struct {
	code string
	day  time.Time
	hash string
}

type visitorDimHash struct {
	code       string
	day        time.Time
	dim, value string
	hash       string
}

func strVal(values map[string]interface{}, key string) string {
	v, ok := values[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// locationField resolves a visit's IP to country/region/city, or "unknown"
// each when no resolver is configured or the IP is unresolvable.
func locationField(g GeoResolver, ip string) (string, string, string) {
	if g == nil {
		return "unknown", "unknown", "unknown"
	}
	loc := g.Lookup(ip)
	return orUnknown(loc.Country), orUnknown(loc.Region), orUnknown(loc.City)
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func values[K comparable, V any](m map[K]V) []V {
	out := make([]V, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
