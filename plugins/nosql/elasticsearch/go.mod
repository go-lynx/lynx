module github.com/go-lynx/lynx/plugins/nosql/elasticsearch

go 1.24.3

require (
	github.com/go-lynx/lynx v0.0.0
	github.com/elastic/go-elasticsearch/v8 v8.12.0
	github.com/go-kratos/kratos/v2 v2.8.4
	github.com/prometheus/client_golang v1.23.0
	google.golang.org/protobuf v1.36.6
)

replace github.com/go-lynx/lynx => ../../../
