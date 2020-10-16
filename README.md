# STAN: Event Stream Concepts
NATS Streaming Concepts using ASCII Art

## Purpose

While evaluating the use of NATS Streaming (STAN), we wanted a way to demonstrate and visualize the differences in both 
the publishing and subscription behaviors available when using At Least Once delivery. While this could have been more 
simply accomplished by printing a sequences of numbers (and, you know, maybe even more practically) - it seemed more 
engaging to demonstrate the differences in visual manner.

To do that, this demo uses a [neat open source project](https://github.com/qeesung/image2ascii) to take a given image and 
convert it to ASCII Art - then it streams the ASCII characters as individual messages through NATS Streaming.  Consumers 
of the messages simply print the message body to stdout.

We can use the output ASCII Art to show how Queue Groups split messages, failed acks cause duplicates or dropped messages
can cause out of sequence delivery. This demo also shows how you can do things like buffering to maintain sequence even
with failures.

![](examples/images/basic-example.gif)

## Getting Started

### Installation

You need to have a working NATS Streaming server running. This can be run easily locally by downloading and running the binary.
You can start NATS Streaming with default options locally and it will use in-memory storage and work out of the box.

See: [Running NATS Streaming](https://docs.nats.io/nats-streaming-server/run).

For the demo, you can run `go install github.com/kmfk/stan-demo` and the `stan-demo` binary will be added to your
`$GOPATH/bin` directory - as long as that's properly been set in your $PATH, you can run the binary directly.  No other
files or dependencies will be installed on your system.

### Removing

If you want to remove the binary, you can run clean: `go clean -i github.com/kmfk/stan-demo` - this
will remove the binary from your `$GOPATH/bin`.

### Environment Variables

If you have non-default values (ie, not running STAN locally) you can use environment variables for connection info.
While the Connection String and Cluster name for NATS / NATS Streaming can be passed to all CLI commands, setting these into
the environment should be more convenient. The two environment variables are:
```
NATS_CONNECTION_STRING=nats://0.0.0.0:4222
NATS_CLUSTER=demo
```

## Examples

See the given Examples for ideas on how to experiment with NATS Streaming and understand the different cases covered. While
each example given provides an example gif, all of these examples can be run locally on your own machine and
the necessary CLI commands are included.  

- [Publishing](examples/publishing.md)
- [Subscription Types](examples/subscription_types.md)
- [Errors & Handling Them](examples/errors_and_handling.md)
- [Replaying Events](examples/starting-replaying-events.md)

## CLI Commands

There are two commands that can be run, either `producer` or `consumer` - each having their own options. 

```
#❯ stan-demo

Usage: stan-demo <command> <options...>

Supported commands are: 
producer - Take image files and produce ASCII characters as messages into STAN
consumer - Listen for messages broadcast from the producer.
```

### Producer

The Producer is responsible for taking an image, either a local file (`-file`) or remote url (`-remote`), and converting 
the given image to ASCII format via the [image2ascii](https://github.com/qeesung/image2ascii) library. The ASCII 
characters are then published as messages to NATS Streaming through the [stan.go](https://github.com/nats-io/stan.go) 
client.

```
Usage: stan-demo producer -file <image-file>
Options:
  -batch int
    	Amount of characters sent in each message. 
    	Defaults to 1 (default 1)
  -client string
    	The Nats Streaming Client ID. 
    	Defaults to ascii-producer
  -cluster string
    	The Nats Streaming cluster name. 

    	Can be set from the NATS_CLUSTER environment variable
  -delay duration
      	Adds artificial delay to publishing - set to 0 to disable the delay. When running stanlocally and memory back, its actually a bit too fast for good visual effect. 
      	Defaults to 3ms
  -file string
    	A local image file to be converted
  -inflight int
    	Max Acks In Flight. 
    	Defaults to 16384 (default 16384)
  -nats-url string
    	The NATS Connection String. 
    	
    	Can be set from the NATS_CONNECTION_STRING environment variable. 
    	 Defaults to nats://0.0.0.0:4222
  -ratio float
    	The size ratio of the image, between 0.0 and 1. 
    	
    	The larger the size, the more messages generated - however, because the size of each 
    	character is significantly larger then a pixel, this value should typically be a 
    	fraction of the 1:1 size, ie 0.1 or 0.2. 
    	
    	Defaults to 0.08
  -remote string
    	A remote image file to be converted
  -subject string
    	The subject to publish to. 
    	Defaults to ascii
  -sync
    	Whether messages should be published synchronously. 
    	Defaults to false.
```
 
### Consumer
 
 The Consumer generates a Subscription to listen on the given subject and simply publishes the messages to the console. 
 There are various options which can control how the Subscription is created, including whether its part of a Queue Group,
 whether its Durable and whether closing the connection should either simply close the subscription, or unsubscribe.
 
 ```
Usage: stan-demo consumer -subject <subject>
Options:
  -ack-fail-percent float
    	A percentage of Acks that should artificially fail - useful for testing receiving duplicate messages.
    	Defaults to 0
  -ackwait int
    	The wait time in seconds for the subscriber to manually acknowledge messages. 
    	Defaults to 10
  -buffer
    	Use a custom buffer on STAN Sequence ID to preserve ordering on out of sequence and/or duplicate messages.
    	Defaults to false
  -client string
    	The Nats Streaming Client ID. 
    	Defaults to ascii-producer
  -cluster string
    	The Nats Streaming cluster name.
    	Can be set from the NATS_CLUSTER environment variable
  -drop-percent float
    	A percentage of messages that should be artificially dropped - useful for testing resent messages 
    	hat arrive out of order. 
    	Defaults to 0
  -durable string
    	The name to use for a durable subscription
  -inflight int
    	The total messages allowed in flight for the consumer, awaiting to be Ack'd. 
    	Defaults to 1000 (default 1000)
  -nats-url string
    	The NATS Connection String. 
    	Can be set from the NATS_CONNECTION_STRING environment variable. 
    	Defaults to nats://0.0.0.0:4222
  -offset string
    	The starting offset - allows 'now' or 'all'. If a starting sequence is given, this is ignored. 
    	- 'now' sets the Subscription to receive messages from current time, forward.
    	- 'all' will set the Subscription to receive all messages available for the topic. (default "now")
  -queue-group string
    	A queue group name - starts the subscriber as part of a QueueGroup
  -republish string
    	The name of a new Subject that this consumer will publish all received messages to.
    	This can help to show a pattern that fans out via a Queue Group and then 
    	consolidates all messages.
  -sequence offset
    	A starting Sequence ID for the Subscription. Setting this value will override the offset option above.
  -subject string
    	The subject to publish to. 
    	Defaults to ascii
  -unsubscribe
    	Whether subscriptions should be removed (true) or closed (false).
    	This really only affects Durable subscriptions. 
    	Defaults to false
```