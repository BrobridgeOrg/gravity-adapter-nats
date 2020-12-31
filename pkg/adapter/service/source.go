package adapter

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	eventbus "github.com/BrobridgeOrg/gravity-adapter-nats/pkg/eventbus/service"
	dsa "github.com/BrobridgeOrg/gravity-api/service/dsa"
	gravity_adapter "github.com/BrobridgeOrg/gravity-sdk/adapter"
	parallel_chunked_flow "github.com/cfsghost/parallel-chunked-flow"
	jsoniter "github.com/json-iterator/go"
	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
)

var counter uint64

// Default settings
var DefaultWorkerCount int = 128
var DefaultPingInterval int64 = 10
var DefaultMaxPingsOutstanding int = 3
var DefaultMaxReconnects int = -1

type Packet struct {
	EventName string      `json:"event"`
	Payload   interface{} `json:"payload"`
}

type Source struct {
	adapter             *Adapter
	connector           *gravity_adapter.AdapterConnector
	workerCount         int
	incoming            chan []byte
	eventBus            *eventbus.EventBus
	name                string
	host                string
	port                int
	channel             string
	pingInterval        int64
	maxPingsOutstanding int
	maxReconnects       int
	parser              *parallel_chunked_flow.ParallelChunkedFlow
}

/*
var packetPool = sync.Pool{
	New: func() interface{} {
		return &Packet{}
	},
}
*/
var requestPool = sync.Pool{
	New: func() interface{} {
		return &dsa.PublishRequest{}
	},
}

func StrToBytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

func NewSource(adapter *Adapter, name string, sourceInfo *SourceInfo) *Source {

	// required channel
	if len(sourceInfo.Channel) == 0 {
		log.WithFields(log.Fields{
			"source": name,
		}).Error("Required channel")

		return nil
	}

	info := sourceInfo

	// default settings
	if info.PingInterval == nil {
		info.PingInterval = &DefaultPingInterval
	}

	if info.MaxPingsOutstanding == nil {
		info.MaxPingsOutstanding = &DefaultMaxPingsOutstanding
	}

	if info.MaxReconnects == nil {
		info.MaxReconnects = &DefaultMaxReconnects
	}

	if info.WorkerCount == nil {
		info.WorkerCount = &DefaultWorkerCount
	}

	// Initialize parapllel chunked flow
	pcfOpts := parallel_chunked_flow.Options{
		BufferSize: 204800,
		ChunkSize:  512,
		ChunkCount: 512,
		Handler: func(data interface{}, output chan interface{}) {
			/*
				id := atomic.AddUint64((*uint64)(&counter), 1)
				if id%1000 == 0 {
					log.Info(id)
				}
			*/
			/*
				// Parse JSON
				packet := packetPool.Get().(*Packet)
				err := json.Unmarshal(data.([]byte), packet)
				if err != nil {
					packetPool.Put(packet)
					return
				}
					// Convert payload to JSON string
					payload, err := json.Marshal(packet.Payload)
					if err != nil {
						packetPool.Put(packet)
						return
					}
			*/
			eventName := jsoniter.Get(data.([]byte), "event").ToString()
			payload := jsoniter.Get(data.([]byte), "payload").ToString()
			//			log.Info(test)

			// Preparing request
			request := requestPool.Get().(*dsa.PublishRequest)
			//request.EventName = packet.EventName
			request.EventName = eventName
			request.Payload = StrToBytes(payload)
			//			packetPool.Put(packet)

			output <- request
		},
	}

	return &Source{
		adapter:     adapter,
		connector:   gravity_adapter.NewAdapterConnector(),
		workerCount: *info.WorkerCount,
		incoming:    make(chan []byte, 204800),
		//		requests:            make(chan *dsa.PublishRequest, 102400),
		name:                name,
		host:                info.Host,
		port:                info.Port,
		channel:             info.Channel,
		pingInterval:        *info.PingInterval,
		maxPingsOutstanding: *info.MaxPingsOutstanding,
		maxReconnects:       *info.MaxReconnects,
		parser:              parallel_chunked_flow.NewParallelChunkedFlow(&pcfOpts),
	}
}

func (source *Source) InitSubscription() error {

	log.WithFields(log.Fields{
		"source":      source.name,
		"client_name": source.adapter.clientName + "-" + source.name,
		"channel":     source.channel,
		"count":       source.workerCount,
	}).Info("Initializing subscribers ...")

	// Subscribe with channel name
	natsConn := source.eventBus.GetConnection()
	sub, err := natsConn.Subscribe(source.channel, func(msg *nats.Msg) {
		source.incoming <- msg.Data
	})
	if err != nil {
		log.Warn(err)
	}

	sub.SetPendingLimits(-1, -1)
	natsConn.Flush()

	return nil
}

func (source *Source) Init() error {

	address := fmt.Sprintf("%s:%d", source.host, source.port)

	log.WithFields(log.Fields{
		"source":      source.name,
		"address":     address,
		"client_name": source.adapter.clientName + "-" + source.name,
		"channel":     source.channel,
	}).Info("Initializing source connector")

	// Initializing gravity adapter connector
	opts := gravity_adapter.NewOptions()
	err := source.connector.Connect(address, opts)
	if err != nil {
		return err
	}
	/*
		// Initializing gRPC streams
		p := source.adapter.app.GetGRPCPool()

		// Register initializer for stream
		p.SetStreamInitializer("publish", func(conn *grpc.ClientConn) (interface{}, error) {
			client := dsa.NewDataSourceAdapterClient(conn)
			return client.PublishEvents(context.Background())
		})
	*/
	options := eventbus.Options{
		ClientName:          source.adapter.clientName + "-" + source.name,
		PingInterval:        time.Duration(source.pingInterval),
		MaxPingsOutstanding: source.maxPingsOutstanding,
		MaxReconnects:       source.maxReconnects,
	}

	source.eventBus = eventbus.NewEventBus(
		address,
		eventbus.EventBusHandler{
			Reconnect: func(natsConn *nats.Conn) {
				log.Warn("re-connected to event server")
			},
			Disconnect: func(natsConn *nats.Conn) {
				log.Error("event server was disconnected")
			},
		},
		options,
	)

	err = source.eventBus.Connect()
	if err != nil {
		return err
	}

	go source.eventReceiver()
	go source.requestHandler()

	return source.InitSubscription()
}

func (source *Source) eventReceiver() {

	log.WithFields(log.Fields{
		"source":      source.name,
		"client_name": source.adapter.clientName + "-" + source.name,
		"channel":     source.channel,
		"count":       source.workerCount,
	}).Info("Initializing workers ...")

	for {
		select {
		case msg := <-source.incoming:
			//source.HandleEvent(msg)
			source.parser.Push(msg)
		}
	}
}

func (source *Source) requestHandler() {

	for {
		select {
		//case req := <-source.requests:
		case req := <-source.parser.Output():
			source.HandleRequest(req.(*dsa.PublishRequest))
			requestPool.Put(req)
		}
	}
}

func (source *Source) HandleRequest(request *dsa.PublishRequest) {

	source.connector.Publish(request.EventName, request.Payload, nil)
	/*
		// Getting stream from pool
		err := source.adapter.app.GetGRPCPool().GetStream("publish", func(s interface{}) error {

			// Send request
			return s.(dsa.DataSourceAdapter_PublishEventsClient).Send(request)
		})
		if err != nil {
			log.Error("Failed to get available stream:", err)
			return
		}
	*/
}
