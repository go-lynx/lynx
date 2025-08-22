module github.com/go-lynx/lynx/plugins/nosql/mongodb

go 1.24.3

require (
	github.com/go-lynx/lynx v0.0.0
	go.mongodb.org/mongo-driver v1.15.0
	github.com/go-kratos/kratos/v2 v2.8.4
	github.com/prometheus/client_golang v1.23.0
	google.golang.org/protobuf v1.36.6
)

replace github.com/go-lynx/lynx => ../../../
