module github.com/go-lynx/plugin-reids/v2

go 1.23.0

require (
	github.com/go-kratos/kratos/v2 v2.8.3
	github.com/go-lynx/lynx v0.0.0
	google.golang.org/protobuf v1.35.2
)

replace github.com/go-lynx/lynx => ../../../

require (
	github.com/go-playground/form/v4 v4.2.1 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	golang.org/x/sync v0.10.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241209162323-e6fa225c2576 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
