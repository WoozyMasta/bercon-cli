package beparser

import (
	"net"

	"github.com/oschwald/geoip2-golang"
)

// Set country code from geoDB for admin IP
func (a *Admins) SetCountryCode(geoDB *geoip2.Reader) {
	for i := range *a {
		(*a)[i].Country = getCountryCodeFromIP(geoDB, (*a)[i].IP)
	}
}

// Set country code from geoDB for player IP
func (p *Players) SetCountryCode(geoDB *geoip2.Reader) {
	for i := range *p {
		(*p)[i].Country = getCountryCodeFromIP(geoDB, (*p)[i].IP)
	}
}

// Set country code from geoDB for IP bans
func (b *BansIP) SetCountryCode(geoDB *geoip2.Reader) {
	for i := range *b {
		(*b)[i].Country = getCountryCodeFromIP(geoDB, (*b)[i].IP)
	}
}

// Set country code from geoDB for IP bans in global Bans struct
func (b *Bans) SetCountryCode(geoDB *geoip2.Reader) {
	b.IPBans.SetCountryCode(geoDB)
}

// return country code from geoDB or XX for all unexpected variants
func getCountryCodeFromIP(geoDB *geoip2.Reader, ip string) string {
	netIP := net.ParseIP(ip)
	if netIP == nil {
		return "XX"
	}

	country, err := geoDB.Country(netIP)
	if err != nil {
		return "XX"
	}

	if country.Country.IsoCode == "" {
		return "XX"
	}

	return country.Country.IsoCode
}
