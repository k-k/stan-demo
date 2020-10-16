package internal

import (
    "github.com/nats-io/stan.go"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "testing"
    "time"
)


type MockStan struct {
    mock.Mock
    stan.Conn
}

// Stubs the PublishAsync method on the stan.Conn interface
func (mock *MockStan) PublishAsync(subject string, data []byte, ah stan.AckHandler) (string, error) {
    args := mock.Called("foo", data, ah)

    return args.String(0), args.Error(1)
}

// Test that calling our wrapper PublishAsync pushes the given ID into the drainable queue
func TestDrainableStan_PublishAsync(t *testing.T) {
    d := DrainableStan{}

    m := MockStan{}
    m.On("PublishAsync", "foo", []byte("bar"), mock.Anything).Return("abc", nil)

    d.Conn = &m

    if _, err := d.PublishAsync("foo", []byte("bar"), func(id string, err error) {}); err != nil {
        t.Error(err)
    }

    m.AssertExpectations(t)

    assert.Equal(t, len(d.bucket), 1, "queue should have an item")
}

// Test that when our AckHandler is invoked, it removes the returned ID from the drainable queue
func TestDrainableStan_AckHandler(t *testing.T) {
    d := DrainableStan{}

    m := MockStan{}
    m.On("PublishAsync", "foo", []byte("bar"), mock.Anything).Return("abc", nil)

    d.Conn = &m

    ah := d.AckHandler(func(id string, err error) {})

    id, err := d.PublishAsync("foo", []byte("bar"), ah)
    if err != nil {
        t.Error(err)
    }

    m.AssertExpectations(t)

    // Because our AckHandler would need to be called asynchronously from STAN by listening to a reply subject
    // We're gonna simulate that ourselves... you know, by just invoking it...
    ah(id, nil)

    // nailed it.
    assert.Zero(t, len(d.bucket),"queue should be empty")
}

// Test that our DrainAndClose method blocks no longer than the specified timeout
func TestDrainableStan_DrainTimesout(t *testing.T) {
    d := DrainableStan{}

    m := MockStan{}
    m.On("PublishAsync", "foo", []byte("bar"), mock.Anything).Return("abc", nil)

    d.Conn = &m

    ah := d.AckHandler(func(id string, err error) {})

    if _, err := d.PublishAsync("foo", []byte("bar"), ah); err != nil {
        t.Error(err)
    }

    // Timeout for DrainAndClose
    timeout := time.Millisecond

    // We should push into this chain once the drain timeout is reached
    ch := make(chan bool)
    // Let's make sure this test can't block forever
    to := time.After(5 * timeout)

    go func() {
        if err := d.Drain(timeout); err != nil {
            ch <- true
        }
    }()

    for {
        select {
        // Success
        case <-ch:
            return
        // Let's make sure this test can't block forever
        case <-to:
            t.Error("drain failed to timeout!")
        }
    }
}

// Test that the DrainAndClose function stops blocking once the queue is empty
func TestDrainableStan_Drain(t *testing.T) {
    d := DrainableStan{}
    m := MockStan{}
    m.On("PublishAsync", "foo", []byte("bar"), mock.Anything).Return("abc", nil)

    d.Conn = &m

    ah := d.AckHandler(func(id string, err error) {})

    id, err := d.PublishAsync("foo", []byte("bar"), ah)
    if err != nil {
        t.Error(err)
    }

    // Timeout for DrainAndClose
    timeout := 10 * time.Millisecond

    // We should push into this chain once the drain timeout is reached
    ch := make(chan bool)
    go func() {
        if err := d.Drain(timeout); err != nil {
            t.Errorf("failed to drain: %v", err)
        }

        ch <- true
    }()

    // DrainAndClose the queue after a short timeout
    time.AfterFunc(timeout / 2, func() { ah(id, nil) })

    for {
        select {
        // Success
        case <-ch:
            return
        }
    }
}