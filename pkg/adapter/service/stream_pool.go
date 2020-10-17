package adapter

import (
	"sync"
	"time"

	grpc_connection_pool "github.com/cfsghost/grpc-connection-pool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type StreamInitializer func(*grpc.ClientConn) (interface{}, error)

type StreamPool struct {
	grpcPool          *grpc_connection_pool.GRPCPool
	streamInitializer StreamInitializer
	streams           sync.Map
}

func NewStreamPool(grpcPool *grpc_connection_pool.GRPCPool, streamInitializer StreamInitializer) *StreamPool {

	pool := &StreamPool{
		grpcPool:          grpcPool,
		streamInitializer: streamInitializer,
	}

	go pool.gc()

	return pool
}

func (sp *StreamPool) gc() {

	for {

		// Remove unavailable connection
		sp.streams.Range(func(k interface{}, v interface{}) bool {
			conn := k.(*grpc.ClientConn)
			state := conn.GetState()

			// Connection is unavailable
			if state == connectivity.Shutdown || state == connectivity.TransientFailure {
				sp.streams.Delete(k)
			}

			return true
		})

		time.Sleep(5 * time.Second)
	}
}

func (sp *StreamPool) Get() (interface{}, error) {

	// Getting connection
	conn, err := sp.grpcPool.Get()
	if err != nil {
		return nil, err
	}

	// Getting stream by connection
	val, ok := sp.streams.Load(conn)
	if ok {
		return val, nil
	}

	// Initialize stream for connection
	stream, err := sp.streamInitializer(conn)
	if err != nil {
		return nil, err
	}

	sp.streams.Store(conn, stream)

	return stream, nil
}
