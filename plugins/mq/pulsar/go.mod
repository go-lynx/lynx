module github.com/go-lynx/lynx/plugins/mq/pulsar

go 1.21

require (
	github.com/apache/pulsar-client-go v0.12.0
	github.com/go-lynx/lynx v0.0.0
	github.com/go-lynx/lynx/plugins v0.0.0
	google.golang.org/protobuf v1.31.0
)

replace (
	github.com/go-lynx/lynx => ../../../
	github.com/go-lynx/lynx/plugins => ../../
)
