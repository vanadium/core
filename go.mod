module v.io

go 1.16

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2
	github.com/shirou/gopsutil/v3 v3.21.12-0.20211204033703-4c3edcfe56bb
	github.com/vanadium/go-mdns-sd v0.0.0-20181006014439-f1a1ccd1252e
	golang.org/x/crypto v0.0.0-20211215153901-e495a2d5b3d3
	golang.org/x/mod v0.5.1
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	v.io/x/lib v0.1.10
	v.io/x/ref/internal/logger v0.1.1
	v.io/x/ref/lib/flags/sitedefaults v0.1.1
)

replace v.io/x/ref/lib/flags/sitedefaults => ./x/ref/lib/flags/sitedefaults

replace v.io/x/ref/internal/logger => ./x/ref/internal/logger
