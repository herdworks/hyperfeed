package main

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/herdworks/hyperfeed-go/subscriber"
)

func main() {
	var subscriber *subscriber.Subscriber = &subscriber.Subscriber{}
	var uri *url.URL
	var ctx context.Context = context.Background()
	var args []string = os.Args[1:]
	var err error

	// Parse command line arguments
	if len(args) < 1 {
		fmt.Println("Usage: hyperfeed <uri>")
		os.Exit(1)
	}
	if uri, err = url.Parse(args[0]); err != nil {
		fmt.Printf("Error parsing URI %q: %v\n", args[0], err)
		os.Exit(1)
	}

	// Initialize the Subscriber
	if err = subscriber.Init(ctx, uri, nil); err != nil {
		fmt.Printf("Error initializing subscriber: %v\n", err)
		os.Exit(1)
	}
	if err = subscriber.Load(ctx); err != nil {
		fmt.Printf("Error loading subscriber: %v\n", err)
		os.Exit(1)
	}
	if err = subscriber.Refresh(ctx); err != nil {
		fmt.Printf("Error refreshing subscriber: %v\n", err)
		os.Exit(1)
	}
}
