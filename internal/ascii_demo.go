package internal

import (
	"fmt"
	"github.com/nats-io/stan.go"
	"os"
	"time"
)

const (
	DefaultClusterId          string  = "test-cluster"
	DefaultProducerClientId   string  = "ascii-producer"
	DefaultConsumerClientId   string  = "ascii-consumer"
	DefaultSubject            string  = "ascii"
	DefaultConnectionString   string  = "nats://0.0.0.0:4222"
	DefaultMaxPubAcksInflight int     = 16384
	DefaultMaxAcksInFlight    int     = 512
	DefaultImageSizeRatio     float64 = 0.08
	DefaultPublishDelay               = 5 * time.Millisecond
)

type Message struct {
	MessageSeriesId string `json:"message_series_id"`
	MessageId       int    `json:"id"`
	Body            string `json:"body"`
	End             bool   `json:"end,omitempty"`
}

type ConnectionInfo struct {
	Cluster          string
	ClientId         string
	ConnectionString string
	Subject          string
}

func (nc *NatsClient) SetConnection(ci ConnectionInfo) {

	// Check the environment before using the default value
	if ci.Cluster == "" {
		if clusterEnv := os.Getenv("NATS_CLUSTER"); clusterEnv != "" {
			ci.Cluster = clusterEnv
		} else {
			ci.Cluster = DefaultClusterId
		}
	}

	// Check the environment before using the default value
	if ci.ConnectionString == "" {
		if connectionStringEnv := os.Getenv("NATS_CONNECTION_STRING"); connectionStringEnv != "" {
			ci.ConnectionString = connectionStringEnv
		} else {
			ci.ConnectionString = DefaultConnectionString
		}
	}

	nc.conn = ci
}

type NatsClient struct {
	DrainableStan
	conn ConnectionInfo
}

// Connect to NATS Streaming
func (nc *NatsClient) Connect() error {
	fmt.Println("\n\nCONNECTION DETAILS")
	fmt.Println(
		"\nCluster ID:\t\t", nc.conn.Cluster,
		"\nClient ID:\t\t", nc.conn.ClientId,
		"\nNATS Server Url:\t", nc.conn.ConnectionString,
		"\nChannel:\t\t", nc.conn.Subject,
	)

	client, err := stan.Connect(nc.conn.Cluster, nc.conn.ClientId, stan.NatsURL(nc.conn.ConnectionString))
	if err != nil {
		return fmt.Errorf("\nerror connecting to NATS Server: %v", err)
	}

	nc.Conn = client

	return nil
}
