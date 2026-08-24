package geo

import (
	"net"

	"github.com/oschwald/geoip2-golang"
)

// Resolver looks up a Location from an IP using an offline MaxMind GeoLite2
// database. All lookups are in-memory against a file on disk; no IP ever
// leaves the box.
type Resolver struct {
	db *geoip2.Reader
}

// Location is the derived geographic attribution of a visit. Only these
// strings are ever persisted — never the IP itself.
type Location struct {
	Country string
	Region  string
	City    string
}

// Open opens the GeoLite2-City mmdb at path.
func Open(path string) (*Resolver, error) {
	db, err := geoip2.Open(path)
	if err != nil {
		return nil, err
	}
	return &Resolver{db: db}, nil
}

func (r *Resolver) Close() error { return r.db.Close() }

// Lookup returns the country (ISO code), region, and city for an IP.
// Unresolvable or private IPs return empty fields; callers bucket them as
// "unknown".
func (r *Resolver) Lookup(ip string) Location {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return Location{}
	}
	rec, err := r.db.City(parsed)
	if err != nil {
		return Location{}
	}
	loc := Location{Country: rec.Country.IsoCode}
	if len(rec.Subdivisions) > 0 {
		loc.Region = rec.Subdivisions[0].Names["en"]
	}
	loc.City = rec.City.Names["en"]
	return loc
}
