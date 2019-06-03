module v.io

require (
	github.com/DATA-DOG/go-sqlmock v1.3.2
	github.com/StackExchange/wmi v0.0.0-20181212234831-e0a55b97c705 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/golang/protobuf v1.3.0
	github.com/google/uuid v1.1.1 // indirect
	github.com/gorilla/websocket v1.4.0
	github.com/kr/pty v1.1.3
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/pborman/uuid v1.2.0
	github.com/shirou/gopsutil v2.18.12+incompatible
	github.com/stretchr/testify v1.3.0 // indirect
	github.com/vanadium/go-mdns-sd v0.0.0-20181006014439-f1a1ccd1252e
	golang.org/x/crypto v0.0.0-20190228161510-8dd112bcdc25
	golang.org/x/net v0.0.0-20190301231341-16b79f2e4e95
	golang.org/x/oauth2 v0.0.0-20190226205417-e64efc72b421
	golang.org/x/sys v0.0.0-20190302025703-b6889370fb10 // indirect
	google.golang.org/api v0.1.0
	v.io/x/lib v0.1.3
	v.io/x/ref/lib/flags/sitedefaults v0.1.1
)

replace v.io/x/ref/lib/flags/sitedefaults => ./x/ref/lib/flags/sitedefaults
