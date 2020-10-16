package internal

import (
    "github.com/stretchr/testify/assert"
    "testing"
    "time"
)

// Tests that Add will instantiate the buffer if it has been and properly adds an item
func TestMessageSequenceBuffer_Add(t *testing.T) {
    mb := MessageSequenceBuffer{}

    assert.Empty(t, mb.buffer)

    if err := mb.Add(123, Message{}); err != nil {
        t.Error("failed to add message to buffer")
    }

    assert.Equal(t, 1, len(mb.buffer))
}

// Tests that Add will instantiate the buffer if it has been and properly adds an item
func TestMessageSequenceBuffer_AddErrorsOnPreviousSequenceId(t *testing.T) {
    mb := MessageSequenceBuffer{}

    assert.Empty(t, mb.buffer)

    if err := mb.Add(5, Message{}); err != nil {
        t.Error("failed to add message to buffer")
    }

    // Tick interval while we wait to initialize the buffer
    delay := time.Millisecond
    // Tick interval that we attempt to consume messages from the buffer
    interval := time.Millisecond

    // consume the buffer
    mb.Consume(delay, interval)

    ch := make(chan bool)
    time.AfterFunc(2 * delay, func() {
        if err := mb.Add(1, Message{}); err != nil {
            ch <- true
        }
    })

    // Let's make sure this test can't block forever
    timeout := time.After(5 * delay)

    for {
        select {
        case <-ch:
            return
        case <-timeout:
            t.Error("timeout reached - failed to consume messages!")
            return
        }
    }
}

// Test that the Consume function will wait for messages to exist in the buffer
func TestMessageSequenceBuffer_ConsumeWaitsForMessages(t *testing.T) {
    mb := MessageSequenceBuffer{}

    // Tick interval while we wait to initialize the buffer
    delay := 3 * time.Millisecond
    // Tick interval that we attempt to consume messages from the buffer
    interval := time.Millisecond

    // Let's make sure this test can't block forever
    timeout := time.After(5 * delay)
    // consume the buffer
    ch := mb.Consume(delay, interval)

    // Add a message eventually
    time.AfterFunc(4 * interval, func() { _ = mb.Add(123, Message{}) })

    for {
        select {
        // Success
        case <-ch:
            return
        case <-timeout:
            t.Error("timeout reached - failed to consume messages!")
            return
        }
    }
}

// Test that the Consume function will wait for messages to exist in the buffer
func TestMessageSequenceBuffer_ConsumeStartsWithLowestId(t *testing.T) {
    mb := MessageSequenceBuffer{}

    // intentionally add these out of order
    _ = mb.Add(100, Message{ MessageId: 1, Body: "too big"})
    _ = mb.Add(1000, Message{ MessageId: 2, Body: "way too big"})
    _ = mb.Add(10, Message{ MessageId: 2, Body: "just right"})

    // Tick interval while we wait to initialize the buffer
    delay := time.Millisecond
    // Tick interval that we attempt to consume messages from the buffer
    interval := time.Millisecond

    // Let's make sure this test can't block forever
    timeout := time.After(5 * delay)
    // consume the buffer
    ch := mb.Consume(delay, interval)

    for {
        select {
        case msg := <-ch:
            // Success
            assert.Equal(t, Message{ MessageId: 2, Body: "just right"}, msg, "should be just right")
            return
        case <-timeout:
            t.Error("timeout reached - failed to consume messages!")
            return
        }
    }
}

// Test that the Consume function will wait for messages to exist in the buffer
func TestMessageSequenceBuffer_ConsumeWaitsMissingMessages(t *testing.T) {
    mb := MessageSequenceBuffer{}

    // intentionally add these out of order
    _ = mb.Add(1, Message{ MessageId: 1, Body: "1" })
    _ = mb.Add(2, Message{ MessageId: 2, Body: "2" })
    _ = mb.Add(4, Message{ MessageId: 4, Body: "4" })

    // Tick interval while we wait to initialize the buffer
    delay := 3 * time.Millisecond
    // Tick interval that we attempt to consume messages from the buffer
    interval := time.Millisecond

    // Let's make sure this test can't block forever
    timeout := time.After(10 * delay)
    // consume the buffer
    ch := mb.Consume(delay, interval)

    // Add 3 eventually
    time.AfterFunc(2 * interval, func() { _ = mb.Add(3, Message{ MessageId: 3, Body: "3" })})

    var results []int
    out:
    for {
        select {
        case msg := <-ch:
            if len(results) > 0 && results[len(results) - 1] != msg.MessageId - 1 {
                t.Error("messages delivered out of order!")
            }

            results = append(results, msg.MessageId)

            if len(results) == 4 {
                break out
            }
        case <-timeout:
            t.Error("timeout reached - failed to consume messages!")
            return
        }
    }

    assert.EqualValues(t, []int{1, 2, 3, 4}, results, "messages should be in order")
}