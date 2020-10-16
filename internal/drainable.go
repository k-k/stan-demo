package internal

import (
	"errors"
	"fmt"
	"github.com/nats-io/stan.go"
	"sync"
	"time"
)

type Drainable interface {
	// Wraps stan.PublishAsync to automatically add the returned NUID into the Drainable Queue
	PublishAsync(subject string, data []byte, fn stan.AckHandler) (string, error)

	// Wraps the given stan.AckHandler to automatically remove the acknowledged NUID from the Drainable Queue
	AckHandler(fn stan.AckHandler) stan.AckHandler

	// Blocks until any inflight messages (based on the queue) have been acknowledged - will timeout after
	// `timeout` duration has been reached.
	Drain(timeout time.Duration) error
}

type DrainableStan struct {
	stan.Conn

	m      sync.Mutex
	bucket map[string]struct{}
}

// Adds the NUID from STAN Streaming to the Drainable Queue
func (d *DrainableStan) add(id string) {
	defer d.m.Unlock()

	d.m.Lock()
	if d.bucket == nil {
		d.bucket = map[string]struct{}{}
	}
	d.bucket[id] = struct{}{}
}

// Removes the NUID from STAN Streaming from the Drainable Queue
func (d *DrainableStan) remove(id string) {
	defer d.m.Unlock()

	d.m.Lock()
	delete(d.bucket, id)
}

// Wraps stan.PublishAsync to automatically add the returned NUID into the Drainable Queue
// Calling this will invoke stan.PublishAsync and if that returns with out error, will add the given NUID to the queue
func (d *DrainableStan) PublishAsync(subject string, data []byte, fn stan.AckHandler) (string, error) {
	id, err := d.Conn.PublishAsync(subject, data, fn)
	if err == nil {
		d.add(id)
	}

	return id, err
}

// Wraps the stan.AckHandler to automatically remove the acknowledged NUID from the Drainable Queue
// Calling this will remove the given NUID from the queue before invoking the wrapped ack handler
func (d *DrainableStan) AckHandler(fn stan.AckHandler) stan.AckHandler {
	return func(s string, e error) {
		d.remove(s)
		fn(s, e)
	}
}

// Blocks until any inflight messages (based on the queue) have been acknowledged - will timeout after `timeout`
// duration has been reached.
func (d *DrainableStan) Drain(timeout time.Duration) error {
	t := time.NewTicker(5 * time.Millisecond)
	to := time.NewTimer(timeout)

	for {
		select {
		case <-t.C:
			if len(d.bucket) <= 0 {
				return nil
			}
		case <-to.C:
			return errors.New(fmt.Sprintf("timed out waiting for %d acks", len(d.bucket)))
		}
	}
}
