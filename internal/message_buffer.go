package internal

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// A Message Buffer allows for adding messages to a queue, then choosing when and how they are consumed. This is primarily
// to allow for forcing in-order processing of messages.
type MessageBuffer interface {
	// Add a Message to the buffer, requires an identifier and the message body
	Add(i interface{}, val interface{}) error

	// Consume returns a channel which messages will be pumped out onto based on the given `interval`.
	//  - `delay` is used only when the MessageBuffer is created in order to provide a window for messages to arrive
	//     before determining which message is the starting point.
	//  - `interval` controls the frequency at which a message is attempted to be pumped into the channel.
	Consume(delay time.Duration, interval time.Duration) chan Message
}

// In memory buffer which uses the NATS Streaming Sequence ID for sorting
type MessageSequenceBuffer struct {
	m       sync.Mutex
	current uint64
	buffer  map[uint64]Message
}

// Used by the Consume method when buffering starts in order to find the lowest sequence id in the current
// set of messages - when there are messages in the buffer.
func (b *MessageSequenceBuffer) init() {
	defer b.m.Unlock()

	var x uint64

	b.m.Lock()

	if b.buffer == nil || len(b.buffer) == 0 {
		return
	}

	for key := range b.buffer {
		if x == 0 || key < x {
			x = key
		}
	}

	b.current = x
}

// Removes the message from the MessageSequenceBuffer
func (b *MessageSequenceBuffer) pop() (Message, error) {
	defer b.m.Unlock()

	b.m.Lock()
	if val, ok := b.buffer[b.current]; ok {
		delete(b.buffer, b.current)

		b.current++

		return val, nil
	}

	return Message{}, errors.New("sequence item does not exist")
}

// Adds a message into the MessageSequenceBuffer based on the NATS Streaming Sequence ID
func (b *MessageSequenceBuffer) Add(i uint64, val Message) error {
	defer b.m.Unlock()

	b.m.Lock()

	if b.current > i {
		return fmt.Errorf("given sequence id is lower than the current sequence, current: %d, given: %d", b.current, i)
	}

	if b.buffer == nil {
		b.buffer = map[uint64]Message{}
	}

	b.buffer[i] = val

	return nil
}

// Consumes messages from the MessageSequenceBuffer
// This will continue to attempt to initialize the buffer using the `delay` as a Ticker to continue to wait in case
// the buffer is empty after the initial `delay` duration has passed.
func (b *MessageSequenceBuffer) Consume(delay time.Duration, interval time.Duration) chan Message {
	ch := make(chan Message)

	go func() {
		ticker := time.NewTicker(delay)

		for {
			select {
			case <-ticker.C:
				b.init()
				if b.current != 0 {
					return
				}
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(interval)

		for {
			select {
			case <-ticker.C:
				if b.current == 0 {
					continue
				}

				// Output as many messages as we can if they're in order and ready
				for len(b.buffer) > 0 {
					if val, err := b.pop(); err == nil {
						ch <- val
					} else {
						// Try again next tick
						break
					}
				}
			}
		}
	}()

	return ch
}
