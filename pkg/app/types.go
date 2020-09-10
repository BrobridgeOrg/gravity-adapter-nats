package app

import (
	"github.com/BrobridgeOrg/gravity-adapter-nats/pkg/grpcbus"
)

type App interface {
	GetGRPCPool() grpcbus.GRPCPool
}