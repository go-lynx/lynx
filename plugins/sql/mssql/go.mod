module github.com/go-lynx/lynx/plugins/sql/mssql

go 1.24.3

toolchain go1.24.4

require (
	github.com/go-lynx/lynx v1.2.1
	github.com/go-lynx/lynx/plugins/sql/base v0.0.0
	github.com/prometheus/client_golang v1.23.0
	google.golang.org/protobuf v1.36.6
)

replace (
	github.com/go-lynx/lynx => ../../../
	github.com/go-lynx/lynx/plugins/sql/base => ../base
)
