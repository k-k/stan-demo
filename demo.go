package main

import (
	"flag"
	"fmt"
	"github.com/kmfk/stan-demo/cmd"
	"github.com/kmfk/stan-demo/internal"
	"os"
)

func main() {
	flag.Usage = func() {
		fmt.Println(fmt.Sprintf("\nUsage: %s <command> <options...>", os.Args[0]))
		fmt.Println("")
		fmt.Println("Supported commands are: ")
		fmt.Println("producer - Take image files and produce ascii characters into STAN")
		fmt.Println("consumer - Listen for messages broad casted from the producer.")
	}

	if len(os.Args) == 1 {
		flag.Usage()
		return
	}

	switch os.Args[1] {
	case "consumer":
		c := flag.NewFlagSet("consumer", flag.ExitOnError)
		opts := internal.ConsumerOptions{}
		cmd.ConsumerFlags(c, &opts)

		if len(os.Args) == 3 && os.Args[2] == "help" {
			c.Usage()
			return
		}

		if err := c.Parse(os.Args[2:]); err != nil {
			fmt.Println(err)
			return
		}

		if err := cmd.Consumer(&opts); err != nil {
			fmt.Println(err)
			return
		}
	case "producer":
		p := flag.NewFlagSet("producer", flag.ExitOnError)
		opts := internal.ProducerOptions{}
		cmd.ProducerFlags(p, &opts)

		if len(os.Args) == 3 && os.Args[2] == "help" {
			p.Usage()
			return
		}

		if err := p.Parse(os.Args[2:]); err != nil {
			fmt.Println(err)
			return
		}

		if err := cmd.Producer(&opts); err != nil {
			fmt.Println(err)
			return
		}
	default:
		fmt.Printf("%q is not valid command.\n", os.Args[1])
		os.Exit(2)
	}
}
