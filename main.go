package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/herdworks/hyperfeed-go/subscriber"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// ============================ Package Variables ============================
var (
	LibVersion    = hyperfeed.LibVersion
	SchemaVersion = hyperfeed.LibSchemaVersion
	ReleaseDate   = hyperfeed.LibReleaseDate
)

// ============================ Main Entry Point =============================
func main() {
	var subscriber *subscriber.Subscriber = &subscriber.Subscriber{}
	var uri *url.URL
	var ctx context.Context = context.Background()
	var args []string = os.Args[1:]
	var err error
	var offset time.Time = time.Time{} // zero time

	// Parse command line arguments
	if len(args) < 1 {
		fmt.Printf("hyperfeed %s\n", LibVersion)
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
	// Refresh subscriber.SubscriptionList from underlying OPML source
	if err = subscriber.Load(ctx); err != nil {
		fmt.Printf("Error loading subscriber: %v\n", err)
		os.Exit(1)
	}
	// Refresh all feeds
	if err = subscriber.Refresh(ctx, offset); err != nil {
		fmt.Printf("Error refreshing subscriber: %v\n", err)
		os.Exit(1)
	}
}
