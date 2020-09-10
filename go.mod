module v.io

go 1.13

require (
	github.com/DATA-DOG/go-sqlmock v1.3.3
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/protobuf v1.3.2
	github.com/gorilla/websocket v1.4.1
	github.com/kr/pty v1.1.8
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/pborman/uuid v1.2.0
	github.com/shirou/gopsutil v2.19.9+incompatible
	github.com/stretchr/testify v1.4.0 // indirect
	github.com/vanadium/go-mdns-sd v0.0.0-20181006014439-f1a1ccd1252e
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/mod v0.3.0
	golang.org/x/net v0.0.0-20200625001655-4c5254603344
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543 // indirect
	google.golang.org/api v0.9.0
	gopkg.in/yaml.v2 v2.2.8 // indirect
	v.io/x/lib v0.1.5
	v.io/x/ref/lib/flags/sitedefaults v0.1.1
)

replace v.io/x/ref/lib/flags/sitedefaults => ./x/ref/lib/flags/sitedefaults
