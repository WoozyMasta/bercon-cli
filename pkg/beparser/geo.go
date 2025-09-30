package beparser

import (
	"net"

	"github.com/oschwald/geoip2-golang"
)

// lookupGeo tries City() first and falls back to Country().
// Returns ("XX","",0,0) when unknown/invalid.
func lookupGeo(db *geoip2.Reader, ip string) (country, city string, lat, lon float64) {
	country = "XX"
	if db == nil {
		return
	}

	netIP := net.ParseIP(ip)
	if netIP == nil {
		return
	}

	// Try City DB first (if DB supports it)
	if rec, err := db.City(netIP); err == nil && rec != nil {
		if cc := rec.Country.IsoCode; cc != "" {
			country = cc
		}

		if rec.City.Names != nil {
			if name := rec.City.Names["en"]; name != "" {
				city = name
			} else {
				for _, v := range rec.City.Names {
					if v != "" {
						city = v
						break
					}
				}
			}
		}

		lat = rec.Location.Latitude
		lon = rec.Location.Longitude

		return
	}

	// Fallback to Country (supported by City/Country/Enterprise DBs)
	if rec, err := db.Country(netIP); err == nil && rec != nil && rec.Country.IsoCode != "" {
		country = rec.Country.IsoCode
	}
	return
}

// SetGeo enriches each Admin with geolocation (country ISO code,
// best-effort city name, latitude and longitude) using the provided
// GeoIP2/GeoLite2 database. Missing/unknown values are left empty.
func (a *Admins) SetGeo(db *geoip2.Reader) {
	for i := range *a {
		c, city, lat, lon := lookupGeo(db, (*a)[i].IP)
		(*a)[i].Country, (*a)[i].City = c, city
		(*a)[i].Latitude, (*a)[i].Longitude = lat, lon
	}
}

// SetGeo enriches each Player with geolocation (country ISO code,
// best-effort city name, latitude and longitude) using the provided
// GeoIP2/GeoLite2 database. Missing/unknown values are left empty.
func (p *Players) SetGeo(db *geoip2.Reader) {
	for i := range *p {
		c, city, lat, lon := lookupGeo(db, (*p)[i].IP)
		(*p)[i].Country, (*p)[i].City = c, city
		(*p)[i].Latitude, (*p)[i].Longitude = lat, lon
	}
}

// SetGeo enriches each BanIP with geolocation (country ISO code,
// best-effort city name, latitude and longitude) using the provided
// GeoIP2/GeoLite2 database. Missing/unknown values are left empty.
func (b *BansIP) SetGeo(db *geoip2.Reader) {
	for i := range *b {
		c, city, lat, lon := lookupGeo(db, (*b)[i].IP)
		(*b)[i].Country, (*b)[i].City = c, city
		(*b)[i].Latitude, (*b)[i].Longitude = lat, lon
	}
}

// SetGeo enriches the IP bans in Bans with geolocation data.
func (b *Bans) SetGeo(db *geoip2.Reader) {
	b.IPBans.SetGeo(db)
}

// **** Backward-compatible API (only Country)

// SetCountryCode sets only the Country ISO code for each Admin using
// the provided GeoIP database (kept for backward compatibility).
func (a *Admins) SetCountryCode(db *geoip2.Reader) {
	for i := range *a {
		c, _, _, _ := lookupGeo(db, (*a)[i].IP)
		(*a)[i].Country = c
	}
}

// SetCountryCode sets only the Country ISO code for each Player using
// the provided GeoIP database (kept for backward compatibility).
func (p *Players) SetCountryCode(db *geoip2.Reader) {
	for i := range *p {
		c, _, _, _ := lookupGeo(db, (*p)[i].IP)
		(*p)[i].Country = c
	}
}

// SetCountryCode sets only the Country ISO code for each BanIP using
// the provided GeoIP database (kept for backward compatibility).
func (b *BansIP) SetCountryCode(db *geoip2.Reader) {
	for i := range *b {
		c, _, _, _ := lookupGeo(db, (*b)[i].IP)
		(*b)[i].Country = c
	}
}

// SetCountryCode sets only the Country ISO code for the IP bans in Bans
// (kept for backward compatibility).
func (b *Bans) SetCountryCode(db *geoip2.Reader) {
	b.IPBans.SetCountryCode(db)
}
