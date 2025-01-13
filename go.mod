module github.com/woozymasta/bercon-cli

go 1.23.1

require (
	github.com/jessevdk/go-flags v1.6.1
	github.com/oschwald/geoip2-golang v1.11.0
)

require (
	github.com/oschwald/maxminddb-golang v1.13.1 // indirect
	golang.org/x/sys v0.28.0 // indirect
)

replace (
	internal/tableprinter => ./internal/tableprinter
	internal/vars => ./internal/vars
)
