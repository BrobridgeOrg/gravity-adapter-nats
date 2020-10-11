module github.com/BrobridgeOrg/gravity-adapter-nats

go 1.13

require (
	github.com/BrobridgeOrg/gravity-api v0.2.0
	github.com/json-iterator/go v1.1.6
	github.com/nats-io/nats.go v1.10.0
	github.com/sirupsen/logrus v1.2.0
	github.com/spf13/viper v1.7.1
	google.golang.org/grpc v1.31.0
)

//replace github.com/BrobridgeOrg/gravity-api => ../gravity-api
