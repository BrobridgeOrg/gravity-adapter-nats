package adapter

import (
	"context"
	"fmt"
	"sync"
	"time"

	eventbus "github.com/BrobridgeOrg/gravity-adapter-nats/pkg/eventbus/service"
	dsa "github.com/BrobridgeOrg/gravity-api/service/dsa"
	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
)

//var counter uint64 = 0

// Default settings
var DefaultWorkerCount int = 8
var DefaultPingInterval int64 = 10
var DefaultMaxPingsOutstanding int = 3
var DefaultMaxReconnects int = -1

type Packet struct {
	EventName string      `json:"event"`
	Payload   interface{} `json:"payload"`
}

type Source struct {
	adapter             *Adapter
	workerCount         int
	incoming            chan *nats.Msg
	eventBus            *eventbus.EventBus
	name                string
	host                string
	port                int
	channel             string
	pingInterval        int64
	maxPingsOutstanding int
	maxReconnects       int
}

var requestPool = sync.Pool{
	New: func() interface{} {
		return &dsa.PublishRequest{}
	},
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

	return &Source{
		adapter:             adapter,
		workerCount:         *info.WorkerCount,
		incoming:            make(chan *nats.Msg, 4096),
		name:                name,
		host:                info.Host,
		port:                info.Port,
		channel:             info.Channel,
		pingInterval:        *info.PingInterval,
		maxPingsOutstanding: *info.MaxPingsOutstanding,
		maxReconnects:       *info.MaxReconnects,
	}
}

func (source *Source) InitSubscription() error {

	// Subscribe to channel
	natsConn := source.eventBus.GetConnection()

	// Subscribe with channel name
	//_, err := natsConn.Subscribe(source.channel, source.HandleMessage)
	_, err := natsConn.Subscribe(source.channel, func(msg *nats.Msg) {
		source.incoming <- msg
	})
	if err != nil {
		log.Warn(err)
		return err
	}

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
				/*
					err := source.InitSubscription()
					if err != nil {
						log.Error(err)
						return
					}
				*/
				log.Warn("re-connected to event server")
			},
			Disconnect: func(natsConn *nats.Conn) {
				log.Error("event server was disconnected")
			},
		},
		options,
	)

	err := source.eventBus.Connect()
	if err != nil {
		return err
	}

	source.InitConsumers()

	return source.InitSubscription()
}

func (source *Source) InitConsumers() {

	log.WithFields(log.Fields{
		"source":      source.name,
		"client_name": source.adapter.clientName + "-" + source.name,
		"channel":     source.channel,
		"count":       source.workerCount,
	}).Info("Initializing consumers ...")

	// Multiplexing
	for i := 0; i < source.workerCount; i++ {
		go func() {
			for {
				select {
				case msg := <-source.incoming:
					source.HandleMessage(msg)
				}
			}
		}()
	}
}

func (source *Source) HandleMessage(m *nats.Msg) {
	/*
		id := atomic.AddUint64((*uint64)(&counter), 1)
		if id%1000 == 0 {
			log.Info(id)
		}
	*/
	var packet Packet

	// Parse JSON
	err := json.Unmarshal(m.Data, &packet)
	if err != nil {
		return
	}
	/*
		log.WithFields(log.Fields{
			"event": packet.EventName,
		}).Info("Received event")
	*/
	// Convert payload to JSON string
	payload, err := json.Marshal(packet.Payload)
	if err != nil {
		return
	}

	// Preparing request
	request := requestPool.Get().(*dsa.PublishRequest)
	request.EventName = packet.EventName
	request.Payload = string(payload)

	// Getting connection from pool
	conn, err := source.adapter.app.GetGRPCPool().Get()
	if err != nil {
		log.Error("Failed to get connection: ", err)
		return
	}
	client := dsa.NewDataSourceAdapterClient(conn)
	source.adapter.app.GetGRPCPool().Put(conn)

	// Preparing context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Publish
	resp, err := client.Publish(ctx, request)
	if err != nil {

		log.Error("did not connect: ", err)

		// Release
		requestPool.Put(request)

		return
	}

	// Release
	requestPool.Put(request)

	if resp.Success == false {
		log.Error("Failed to push message to data source adapter")
		return
	}
}
