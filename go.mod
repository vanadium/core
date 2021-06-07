module v.io

go 1.13

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/aws/aws-sdk-go v1.38.24 // indirect
	github.com/aws/aws-xray-sdk-go v1.3.0
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/gorilla/websocket v1.4.2
	github.com/kr/pty v1.1.8
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/pborman/uuid v1.2.0
	github.com/shirou/gopsutil v2.19.9+incompatible
	github.com/vanadium/go-mdns-sd v0.0.0-20181006014439-f1a1ccd1252e
	golang.org/x/crypto v0.0.0-20210513122933-cd7d49e622d5
	golang.org/x/mod v0.3.0
	golang.org/x/net v0.0.0-20210510120150-4163338589ed
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/api v0.9.0
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
	google.golang.org/grpc v1.36.0 // indirect
	v.io/x/lib v0.1.7
	v.io/x/ref/internal/logger v0.1.1
	v.io/x/ref/lib/flags/sitedefaults v0.1.1
)

replace v.io/x/ref/lib/flags/sitedefaults => ./x/ref/lib/flags/sitedefaults

replace v.io/x/ref/internal/logger => ./x/ref/internal/logger

replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200519105757-fe76b779f299

replace google.golang.org/grpc => google.golang.org/grpc v1.26.0

replace github.com/golang/protobuf => github.com/golang/protobuf v1.3.3

replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20190801165951-fa694d86fc64
