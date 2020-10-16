package cmd

import (
	"flag"
	"fmt"
	"math"
	"github.com/kmfk/stan-demo/internal"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Creates a Subscriber or QueueSubscriber to a channel in NATS Streaming to listen for ASCII Characters and prints
// them to the console.
func Consumer(opts *internal.ConsumerOptions) error {
	c := internal.Consumer{}

	if err := c.SetOptions(*opts); err != nil {
		return err
	}

	if err := c.Connect(); err != nil {
		return err
	}

	if err := c.CreateSubscription(); err != nil {
		return err
	}

	ctlc := make(chan os.Signal, 2)
	signal.Notify(ctlc, os.Interrupt, syscall.SIGTERM)

	ch := c.Consume()

	var current string
	stats := MessageStats{}

	for {
		select {
		case msg := <-ch:
			if current != msg.MessageSeriesId {
				current = msg.MessageSeriesId
				stats.start = time.Now()
			}

			fmt.Print(msg.Body)
			stats.MessagesReceived++

			if msg.End {
				stats.end = time.Now()

				fmt.Println(fmt.Sprintf(
					"\n Image: %s  |  Total Sent: %d  |  Time Taken: %s  | Messages/Sec: %v \n",
					current,
					stats.MessagesReceived,
					stats.GetDuration(),
					stats.GetMessagesPerSecond(),
				))

				current = ""
			}
		case <-ctlc:
			if err := c.End(); err != nil {
				return err
			}

			stats := c.GetSubscriptionStats()
			fmt.Println("\nTotal Messages:", stats.Received, "| Total Acknowledged:", stats.AcksSent, "| Total Dropped:", stats.FailedAcks)

			return nil
		}
	}
}

func ConsumerFlags(fs *flag.FlagSet, opts *internal.ConsumerOptions) {
	fs.StringVar(&opts.Cluster,
		"cluster",
		"",
		`The Nats Streaming cluster name.
Can be set from the NATS_CLUSTER environment variable`,
	)
	fs.StringVar(&opts.ClientId,
		"client",
		"",
		"The Nats Streaming Client ID. \nDefaults to ascii-producer")
	fs.StringVar(&opts.ConnectionString,
		"nats-url",
		"",
		`The NATS Connection String. 
Can be set from the NATS_CONNECTION_STRING environment variable. 
Defaults to nats://0.0.0.0:4222`,
	)
	fs.StringVar(&opts.Subject,
		"subject",
		"",
		"The subject to publish to. \nDefaults to ascii")
	fs.StringVar(&opts.QueueGroup,
		"queue-group",
		"",
		"A queue group name - starts the subscriber as part of a QueueGroup")
	fs.StringVar(&opts.DurableSubscription,
		"durable",
		"",
		"The name to use for a durable subscription")
	fs.IntVar(&opts.AckWait,
		"ackwait",
		0,
		"The wait time in seconds for the subscriber to manually acknowledge messages. \nDefaults to 10")
	fs.Float64Var(&opts.AckFailPercent,
		"ack-fail-percent",
		0.00,
		`A percentage of Acks that should artificially fail - useful for testing receiving duplicate messages.
Defaults to 0`,
	)
	fs.Float64Var(&opts.MsgDropPercent,
		"drop-percent",
		0.00,
		`A percentage of messages that should be artificially dropped - useful for testing resent messages 
hat arrive out of order. 
Defaults to 0`,
	)
	fs.BoolVar(&opts.BufferMessages,
		"buffer",
		false,
		`Use a custom buffer on STAN Sequence ID to preserve ordering on out of sequence and/or duplicate messages.
Defaults to false`,
	)
	fs.IntVar(&opts.MaxInFlight,
		"inflight",
		1000,
		"The total messages allowed in flight for the consumer, awaiting to be Ack'd. \nDefaults to 1000")
	fs.StringVar(&opts.RepublishSubject,
		"republish",
		"",
		`The name of a new Subject that this consumer will publish all received messages to.
This can help to show a pattern that fans out via a Queue Group and then 
consolidates all messages.`,
	)
	fs.BoolVar(&opts.UnsubscribeOnClose,
		"unsubscribe",
		false,
		`Whether subscriptions should be removed (true) or closed (false).
This really only affects Durable subscriptions. 
Defaults to false`,
	)
	fs.StringVar(&opts.StartingOffset,
		"offset",
		"now",
		`The starting offset - allows 'now' or 'all'. If a starting sequence is given, this is ignored. 
- 'now' sets the Subscription to receive messages from current time, forward.
- 'all' will set the Subscription to receive all messages available for the topic.`,
	)
	fs.Uint64Var(&opts.StartingSequence,
		"sequence",
		0,
		"A starting Sequence ID for the Subscription. Setting this value will override the `offset` option above.")

	fs.Usage = func() {
		fmt.Println(fmt.Sprintf("\nUsage: %s consumer -subject <subject>", os.Args[0]))
		fmt.Println("Options:")
		fs.PrintDefaults()
	}
}

// Basic stats on messages on the Subscriber
type MessageStats struct {
	// Start Time
	start time.Time
	// End Time
	end time.Time
	// How messages were sent
	MessagesReceived int
}

// Return the Time Taken to deliver messages
func (s *MessageStats) GetDuration() time.Duration {
	var d time.Duration
	ms := time.Millisecond

	if !s.start.IsZero() && !s.end.IsZero() {
		d = s.end.Sub(s.start)
	}

	// Lets format the results a bit
	if mod := d % ms; mod+mod < ms {
		d = d - mod
	} else {
		d = d + ms - mod
	}

	return d
}

// Return the Messages Per Second for the Publisher
func (s *MessageStats) GetMessagesPerSecond() float64 {
	if !s.start.IsZero() && !s.end.IsZero() {
		return math.Round(float64(s.MessagesReceived) / s.end.Sub(s.start).Seconds())
	}

	return 0.0
}