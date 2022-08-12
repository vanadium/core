module v.io

go 1.16

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2
	github.com/shirou/gopsutil/v3 v3.21.13-0.20220115084902-511da82e944b
	github.com/vanadium/go-mdns-sd v0.0.0-20181006014439-f1a1ccd1252e
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a
	golang.org/x/crypto v0.0.0-20220112180741-5e0467b6c7ce
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4
	golang.org/x/net v0.0.0-20220722155237-a158d28d115b
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	golang.org/x/text v0.3.7
	golang.org/x/tools v0.1.12 // indirect
	golang.org/x/xerrors v0.0.0-20220411194840-2f41105eb62f // indirect
	v.io/x/lib v0.1.10
	v.io/x/ref/internal/logger v0.1.1
	v.io/x/ref/lib/flags/sitedefaults v0.1.1
	v.io/x/ref/test/compatibility/modules/simple v0.0.0-20220116222041-f948f3a44e0d
)

replace v.io/x/ref/lib/flags/sitedefaults => ./x/ref/lib/flags/sitedefaults

replace v.io/x/ref/internal/logger => ./x/ref/internal/logger

replace v.io/x/ref/test/compatibility/modules/simple => ./x/ref/test/compatibility/modules/simple
