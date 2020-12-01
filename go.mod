module v.io

go 1.13

require (
	github.com/DATA-DOG/go-sqlmock v1.3.3
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/cosnicolaou/go-bindata/v3 v3.0.8 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/protobuf v1.3.2
	github.com/google/uuid v1.0.0
	github.com/gorilla/websocket v1.4.1
	github.com/kr/pty v1.1.8
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/pborman/uuid v1.2.0
	github.com/shirou/gopsutil v2.19.9+incompatible
	github.com/stretchr/testify v1.6.1 // indirect
	github.com/vanadium/go-mdns-sd v0.0.0-20181006014439-f1a1ccd1252e
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/mod v0.3.0
	golang.org/x/net v0.0.0-20201021035429-f5854403a974
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/tools v0.0.0-20201109182053-3db8fd265862 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/api v0.9.0
	v.io/x/lib v0.1.7
	v.io/x/ref/internal/logger v0.1.1
	v.io/x/ref/lib/flags/sitedefaults v0.1.1
)

replace v.io/x/ref/lib/flags/sitedefaults => ./x/ref/lib/flags/sitedefaults

replace v.io/x/ref/internal/logger => ./x/ref/internal/logger
