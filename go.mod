module v.io

go 1.16

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2
	github.com/shirou/gopsutil/v3 v3.21.10
	github.com/vanadium/go-mdns-sd v0.0.0-20181006014439-f1a1ccd1252e
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/mod v0.5.1
	golang.org/x/net v0.0.0-20211101193420-4a448f8816b3
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211031064116-611d5d643895 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	v.io/x/lib v0.1.9
	v.io/x/ref/internal/logger v0.1.1
	v.io/x/ref/lib/flags/sitedefaults v0.1.1
)

replace v.io/x/ref/lib/flags/sitedefaults => ./x/ref/lib/flags/sitedefaults

replace v.io/x/ref/internal/logger => ./x/ref/internal/logger

replace v.io/x/ref/runtime/internal/rpc/benchmark => ./x/ref/runtime/internal/rpc/benchmark

replace v.io/x/ref/examples => ./x/ref/examples/
