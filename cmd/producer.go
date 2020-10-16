package cmd

import (
	"flag"
	"fmt"
	"github.com/kmfk/stan-demo/internal"
	"os"
)

// Uses the given image file and converts to ASCII text and pushes each ASCII character through NATS Streaming.
func Producer(opts *internal.ProducerOptions) error {
	p := internal.Producer{}

	if err := p.SetOptions(*opts); err != nil {
		return err
	}

	if err := p.Connect(); err != nil {
		return err
	}

	if result, err := p.GetAscii(); err != nil {
		return fmt.Errorf("failed to convert image: %v", err)
	} else {
		if err := p.Publish(result); err != nil {
			return fmt.Errorf("stopped due to error: %v", err)
		}
	}

	fmt.Print("\nClosing down... ")
	if err := p.DrainAndClose(); err != nil {
		return err
	} else {
		fmt.Print("done.")
	}

	stats := p.GetPublishStats()
	fmt.Println(fmt.Sprintf(
		"\nTotal Sent: %d  | Total Acks: %d  |  Time Taken: %s  | Messages/Sec: %v",
		stats.MessagesSent,
		stats.AcksReceived,
		stats.GetDuration(),
		stats.GetMessagesPerSecond(),
	))

	return nil
}

func ProducerFlags(fs *flag.FlagSet, opts *internal.ProducerOptions) {
	fs.StringVar(&opts.Cluster,
		"cluster",
		"",
		"The Nats Streaming cluster name. Can be set from the NATS_CLUSTER environment variable")
	fs.StringVar(&opts.ClientId,
		"client",
		"",
		"The Nats Streaming Client ID. \nDefaults to ascii-producer")
	fs.StringVar(&opts.ConnectionString,
		"nats-url",
		"",
		"The NATS Connection String. Can be set from the NATS_CONNECTION_STRING environment variable." +
			"\nDefaults to nats://0.0.0.0:4222")
	fs.StringVar(&opts.LocalFile,
		"file",
		"",
		"A local image file to be converted")
	fs.StringVar(&opts.RemoteFile,
		"remote",
		"",
		"A remote image file to be converted")
	fs.Float64Var(&opts.ImageSizeRatio,
		"ratio",
		0.00,
		"The size ratio of the ASCII image, between 0.0 and 1. The larger the size, the more ASCII characters generated - " +
			"\nBecause the size of each character is significantly larger then a pixel, large values will produce very" +
			"\nlarge ASCII art images." +
			"\nDefaults to 0.08")
	fs.BoolVar(&opts.Sync,
		"sync",
		false,
		"Whether messages should be published synchronously. \nDefaults to false.")
	fs.IntVar(&opts.BatchSize,
		"batch",
		1,
		"Amount of characters sent in each message. \nDefaults to 1")
	fs.StringVar(&opts.Subject,
		"subject",
		"",
		"The subject to publish to. \nDefaults to ascii")
	fs.IntVar(&opts.MaxInFlightAcks,
		"inflight",
		16384,
		"Max Acks In Flight. \nDefaults to 16384")
	fs.DurationVar(&opts.PublishDelay,
		"delay",
		0,
		"Adds artificial delay to publishing - set to 0 to disable the delay. When running stan" +
		"\nlocally and memory back, its actually a bit too fast for good visual effect. \nDefaults to 3ms")

	fs.Usage = func() {
		fmt.Println(fmt.Sprintf("\nUsage: %s producer -file <image-file>", os.Args[0]))
		fmt.Println("Options:")
		fs.PrintDefaults()
	}

	fs.Parse(os.Args[2:])
}
