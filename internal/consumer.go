package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/imdario/mergo"
	"github.com/nats-io/stan.go"
	"math/rand"
	"time"
)

type ConsumerOptions struct {
	// Connection Info
	Cluster          string
	ClientId         string
	ConnectionString string

	Subject             string
	MaxInFlight         int
	QueueGroup          string
	DurableSubscription string
	RepublishSubject    string
	StartingOffset      string
	AckFailPercent      float64
	MsgDropPercent      float64
	StartingSequence    uint64
	AckWait             int

	UnsubscribeOnClose bool
	BufferMessages     bool
}

// Basic stats on messages on the Subscriber
type ConsumerStats struct {
	// How messages are received, regardless of Acks
	Received int
	// Total Acks sent back
	AcksSent int
	// Total Acks failed (intentionally based on AckFailPercent)
	FailedAcks int
	// Total Dropped Messages (intentionally based on MsgDropPercent)
	DroppedMessages int
}

// Opinionated Consumer for the ASCII Messaging Demo
type Consumer struct {
	NatsClient
	options ConsumerOptions
	sub     stan.Subscription
	ch      chan Message
	buffer  MessageSequenceBuffer
	stats   ConsumerStats
}

// Set the options for the Consumer
func (c *Consumer) SetOptions(opts ConsumerOptions) error {

	// Set Default Values
	d := ConsumerOptions{
		ClientId:           DefaultConsumerClientId,
		Subject:            DefaultSubject,
		StartingOffset:     "now",
		AckFailPercent:     0.00,
		MsgDropPercent:     0.00,
		StartingSequence:   0,
		AckWait:            10,
		MaxInFlight:        DefaultMaxAcksInFlight,
		UnsubscribeOnClose: false,
		BufferMessages:     false,
	}

	if err := mergo.Merge(&d, opts, mergo.WithOverride); err != nil {
		return err
	}

	c.SetConnection(ConnectionInfo{
		Cluster:          d.Cluster,
		ClientId:         d.ClientId,
		ConnectionString: d.ConnectionString,
		Subject:          d.Subject,
	})

	c.options = d

	return nil
}

// Returns the currently set options
func (c *Consumer) GetOptions() ConsumerOptions {
	return c.options
}

// Create a Subscription based on the given options.
//
// Creates a Queue Subscription if QueueGroup is set or a Standard Subscription otherwise.
func (c *Consumer) CreateSubscription() error {
	if c.Conn == nil {
		return errors.New("subscription failed, no stan connection")
	}

	if c.ch == nil {
		c.ch = make(chan Message)
	}

	options := []stan.SubscriptionOption{
		stan.MaxInflight(c.options.MaxInFlight),
		stan.AckWait(time.Duration(c.options.AckWait) * time.Second),
		stan.SetManualAckMode(),
	}

	if c.options.DurableSubscription != "" {
		options = append(options, stan.DurableName(c.options.DurableSubscription))
	}

	if c.options.StartingSequence != 0 {
		options = append(options, stan.StartAtSequence(c.options.StartingSequence))
	} else if c.options.StartingOffset == "now" {
		options = append(options, stan.StartAtTime(time.Now()))
	} else if c.options.StartingOffset == "all" {
		options = append(options, stan.DeliverAllAvailable())
	}

	var err error
	if c.options.QueueGroup != "" {
		c.sub, err = c.QueueSubscribe(c.options.Subject, c.options.QueueGroup, c.msgHandler, options...)
	} else {
		c.sub, err = c.Subscribe(c.options.Subject, c.msgHandler, options...)
	}

	if err != nil {
		return fmt.Errorf("failed to create subscription: %v", err)
	}

	return nil
}

// Return the ProducerStats for the subscription
func (c *Consumer) GetSubscriptionStats() ConsumerStats {
	return c.stats
}

// Returns a channel that event messages (ascii characters) will be written to
func (c *Consumer) Consume() chan Message {
	if c.options.BufferMessages {
		return c.buffer.Consume(3*time.Second, 1*time.Millisecond)
	}

	return c.ch
}

// End the Subscription and Connection.
//
// If UnsubscribeOnClose is true, then unsubscribe, removing the subscription from NATS Streaming,
// otherwise, the subscription is closed, but would remain if it was configured as durable.
// Closing a non-durable subscription is the same as unsubscribing.
func (c *Consumer) End() error {
	if c.options.UnsubscribeOnClose {
		fmt.Println("\nUnsubscribing. ")

		err := c.sub.Unsubscribe()
		if err != nil && err != stan.ErrConnectionClosed {
			return fmt.Errorf("error unsubscribing, %v", err)
		}
	} else {
		fmt.Println("\nClosing subscription. ")

		err := c.sub.Close()
		if err != nil && err != stan.ErrConnectionClosed {
			return fmt.Errorf("error closing subscription, %v", err)
		}
	}

	err := c.Close()
	if err != nil && err != stan.ErrConnectionClosed {
		return fmt.Errorf("error closing connection: %v", err)
	}

	return nil
}

// Message Handler used for this Consumer
func (c *Consumer) msgHandler(m *stan.Msg) {
	rand.Seed(time.Now().UnixNano())

	// Simulate a dropped message, forcing STAN to push it back to us after AckWait time
	// The resent message would now arrive out of sequence
	if rand.Float64() <= c.options.MsgDropPercent {
		c.stats.DroppedMessages++
		return
	}

	// Increment that we received the message
	c.stats.Received++

	// Whether we want to republish this same message to another subject
	if c.options.RepublishSubject != "" {
		_, err := c.PublishAsync(c.options.RepublishSubject, m.Data, func(id string, err error) {})
		if err != nil {
			panic(err)
		}
	} else {
		var msg Message
		_ = json.Unmarshal(m.Data, &msg)

		if c.options.BufferMessages {
			c.buffer.Add(m.Sequence, msg)
		} else {
			c.ch <- msg
		}
	}

	// Artificially fail the acknowledgement after handling the message, forcing STAN to push it back to us after
	// AckWait time.  The resent message would now be a duplicate and technically out of sequence
	if rand.Float64() <= c.options.AckFailPercent {
		c.stats.FailedAcks++
		return
	}

	if err := m.Ack(); err != nil {
		c.stats.FailedAcks++
	} else {
		c.stats.AcksSent++
	}
}
