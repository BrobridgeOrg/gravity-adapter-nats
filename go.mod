module github.com/BrobridgeOrg/gravity-adapter-nats

go 1.15

require (
	github.com/BrobridgeOrg/gravity-api v0.2.1
	github.com/cfsghost/grpc-connection-pool v0.6.0
	github.com/cfsghost/parallel-chunked-flow v0.0.2
	github.com/json-iterator/go v1.1.6
	github.com/nats-io/nats-server/v2 v2.1.8 // indirect
	github.com/nats-io/nats.go v1.10.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.4.0 // indirect
	google.golang.org/grpc v1.32.0
//	google.golang.org/grpc v1.31.1
)

//replace github.com/BrobridgeOrg/gravity-api => ../gravity-api

//replace github.com/cfsghost/grpc-connection-pool => /Users/fred/works/opensource/grpc-connection-pool
