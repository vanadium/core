module v.io

go 1.16

require (
	github.com/DATA-DOG/go-sqlmock v1.3.3
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/protobuf v1.4.3
	github.com/google/uuid v1.2.0
	github.com/gorilla/websocket v1.4.2
	github.com/kr/pty v1.1.8
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/pborman/uuid v1.2.0
	github.com/shirou/gopsutil/v3 v3.21.1
	github.com/vanadium/go-mdns-sd v0.0.0-20181006014439-f1a1ccd1252e
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	golang.org/x/mod v0.3.0
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/api v0.9.0
	google.golang.org/grpc v1.36.0 // indirect
	google.golang.org/protobuf v1.25.0
	v.io/x/lib v0.1.8
	v.io/x/ref/internal/logger v0.1.1
	v.io/x/ref/lib/flags/sitedefaults v0.1.1
)

replace v.io/x/ref/lib/flags/sitedefaults => ./x/ref/lib/flags/sitedefaults

replace v.io/x/ref/internal/logger => ./x/ref/internal/logger
