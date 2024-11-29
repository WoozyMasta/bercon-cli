package beparser

import (
	"net"

	"github.com/oschwald/geoip2-golang"
)

func (a *Admins) SetCountryCode(geoDB *geoip2.Reader) {
	for i := range *a {
		(*a)[i].Country = getCountryCodeFromIP(geoDB, (*a)[i].IP)
	}
}

func (p *Players) SetCountryCode(geoDB *geoip2.Reader) {
	for i := range *p {
		(*p)[i].Country = getCountryCodeFromIP(geoDB, (*p)[i].IP)
	}
}

func (b *BansIP) SetCountryCode(geoDB *geoip2.Reader) {
	for i := range *b {
		(*b)[i].Country = getCountryCodeFromIP(geoDB, (*b)[i].IP)
	}
}

func (b *Bans) SetCountryCode(geoDB *geoip2.Reader) {
	b.IPBans.SetCountryCode(geoDB)
}

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
