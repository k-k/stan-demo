package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/imdario/mergo"
	"github.com/nu7hatch/gouuid"
	"github.com/qeesung/image2ascii/convert"
	"image"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type ProducerOptions struct {
	// Connection Info
	Cluster          string `json:"cluster,omitempty"`
	ClientId         string `json:"client_id,omitempty"`
	ConnectionString string `json:"connection_string,omitempty"`

	// Publishing
	Subject         string        `json:"subject,omitempty"`
	MaxInFlightAcks int           `json:"inflight,omitempty"`
	PublishDelay    time.Duration `json:"delay, omitempty"`
	DrainTimeout    time.Duration `json:"drain_timeout,omitempty"`
	BatchSize       int           `json:"batch_size,omitempty"`
	Sync            bool          `json:"sync,omitempty"`

	// File Inputs
	LocalFile      string  `json:"local_file,omitempty"`
	RemoteFile     string  `json:"remote_file"`
	ImageSizeRatio float64 `json:"ratio"`
}

// Basic stats on messages on the Subscriber
type ProducerStats struct {
	// Start Time
	start time.Time
	// End Time
	end time.Time
	// How messages were sent
	MessagesSent int
	// Total Acks received back
	AcksReceived int
}

type Producer struct {
	NatsClient
	options ProducerOptions
	stats   ProducerStats
}

// Set the options for the Producer
func (p *Producer) SetOptions(opts ProducerOptions) error {

	// Set Default Values
	d := ProducerOptions{
		ClientId:        DefaultProducerClientId,
		Subject:         DefaultSubject,
		MaxInFlightAcks: DefaultMaxPubAcksInflight,
		ImageSizeRatio:  DefaultImageSizeRatio,
		DrainTimeout:    60 * time.Second,
		PublishDelay:    DefaultPublishDelay,
		BatchSize:       1,
	}

	if err := mergo.Merge(&d, opts, mergo.WithOverride); err != nil {
		return err
	}

	// mergo won't merge over the top of non-zero values, but we want to allow people
	// to disable the delay, if they want
	if opts.PublishDelay == 0 {
		d.PublishDelay = 0
	}

	// ASCII characters are obviously way larger than pixels, so our
	// ratio is actually only a fraction of the original size
	if d.ImageSizeRatio != DefaultImageSizeRatio {
		d.ImageSizeRatio = d.ImageSizeRatio * 0.1
	}

	p.SetConnection(ConnectionInfo{
		Cluster:          d.Cluster,
		ClientId:         d.ClientId,
		ConnectionString: d.ConnectionString,
		Subject:          d.Subject,
	})

	if d.LocalFile == "" && d.RemoteFile == "" {
		return errors.New("an image file must be specified with `-file` or `-remote`")
	}

	p.options = d

	return nil
}

// Returns the currently set options
func (p *Producer) GetOptions() ProducerOptions {
	return p.options
}

func (p *Producer) GetAscii() ([]string, error) {
	defaultOptions := convert.DefaultOptions
	defaultOptions.Ratio = p.GetOptions().ImageSizeRatio
	converter := convert.NewImageConverter()

	var results []string
	if p.options.RemoteFile != "" {
		img, _, err := downloadImage(p.options.RemoteFile)

		if err != nil {
			return results, err
		}

		results = converter.Image2ASCIIMatrix(img, &defaultOptions)
	} else {
		results = converter.ImageFile2ASCIIMatrix(p.GetOptions().LocalFile, &defaultOptions)
	}

	if p.options.BatchSize > 1 {
		results = batchCharacters(results, p.options.BatchSize)
	}

	return results, nil
}

func batchCharacters(characters []string, chunksize int) []string {
	s := (len(characters) + chunksize - 1) / chunksize
	chunked := make([]string, 0, s)

	for i := 0; i < len(characters); i += chunksize {
		end := i + chunksize

		if end > len(characters) {
			end = len(characters)
		}

		chunked = append(chunked, strings.Join(characters[i:end], ""))
	}

	return chunked
}

func (p *Producer) Publish(messages []string) error {
	defer func() { p.stats.end = time.Now() }()
	p.stats.start = time.Now()

	id, err := uuid.NewV4()
	if err != nil {
		return fmt.Errorf("failed to generate id: %v", err)
	}

	ctlc := make(chan os.Signal, 2)
	signal.Notify(ctlc, os.Interrupt, syscall.SIGTERM)

	var x int
	for pos, char := range messages {
		select {
		case <-ctlc:
			return nil
		default:
			data, err := json.Marshal(Message{
				MessageSeriesId: id.String(),
				MessageId:       pos,
				Body:            char,
				End:             x == len(messages)-1,
			})

			if err != nil {
				return err
			}

			if p.options.Sync {
				if err := p.Conn.Publish(p.options.Subject, data); err != nil {
					return err
				} else {
					p.stats.AcksReceived++
				}

			} else {
				_, err := p.PublishAsync(p.options.Subject, data, p.AckHandler(func(id string, err error) {
					if err != nil {
						fmt.Printf("Oh no! Message %s has errored: %+v\n", id, err)
					}

					p.stats.AcksReceived++
				}))

				if err != nil {
					return err
				}
			}

			x++
			p.stats.MessagesSent++
			time.Sleep(p.options.PublishDelay)
		}
	}

	return nil
}

// Drains the pending Acks from PublishAsync and closes the connection
func (p *Producer) DrainAndClose() error {
	if err := p.Drain(p.options.DrainTimeout); err != nil {
		return fmt.Errorf("error draining: %v", err)
	}

	if err := p.Close(); err != nil {
		return fmt.Errorf("error closing connection: %v", err)
	}

	return nil
}

// Return the ProducerStats for the subscription
func (p *Producer) GetPublishStats() ProducerStats {
	return p.stats
}

// Return the Time Taken to deliver messages
func (s *ProducerStats) GetDuration() time.Duration {
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
func (s *ProducerStats) GetMessagesPerSecond() float64 {
	if !s.start.IsZero() && !s.end.IsZero() {
		return math.Round(float64(s.MessagesSent) / s.end.Sub(s.start).Seconds())
	}

	return 0.0
}

// Download remote images
func downloadImage(url string) (image.Image, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// Keep an in memory copy.
	img, format, err := image.Decode(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return img, format, nil
}
