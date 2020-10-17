package adapter

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/BrobridgeOrg/gravity-adapter-nats/pkg/app"
	dsa "github.com/BrobridgeOrg/gravity-api/service/dsa"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Adapter struct {
	app        app.App
	sm         *SourceManager
	streamPool *StreamPool
	clientName string
}

func NewAdapter(a app.App) *Adapter {
	adapter := &Adapter{
		app: a,
	}

	adapter.sm = NewSourceManager(adapter)

	return adapter
}

func (adapter *Adapter) Init() error {

	adapter.streamPool = NewStreamPool(adapter.app.GetGRPCPool(), adapter.streamInitializer)

	// Using hostname (pod name) by default
	host, err := os.Hostname()
	if err != nil {
		log.Error(err)
		return nil
	}

	host = strings.ReplaceAll(host, ".", "_")

	adapter.clientName = fmt.Sprintf("gravity_adapter_nats-%s", host)

	err = adapter.sm.Initialize()
	if err != nil {
		log.Error(err)
		return nil
	}

	return nil
}

func (adapter *Adapter) streamInitializer(conn *grpc.ClientConn) (interface{}, error) {
	client := dsa.NewDataSourceAdapterClient(conn)
	return client.PublishEvents(context.Background())
}
