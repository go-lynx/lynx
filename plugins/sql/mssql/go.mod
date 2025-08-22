module github.com/go-lynx/lynx/plugins/sql/mssql

go 1.21

require (
	github.com/go-lynx/lynx v0.0.0
	github.com/go-lynx/lynx/plugins/sql/base v0.0.0
	github.com/prometheus/client_golang v1.17.0
	google.golang.org/protobuf v1.31.0
)

replace (
	github.com/go-lynx/lynx => ../../../
	github.com/go-lynx/lynx/plugins/sql/base => ../base
)
