package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	"github.com/herdworks/hyperfeed-go/internal/database"
	"github.com/herdworks/hyperfeed-go/subscriber"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
	version "github.com/herdworks/hyperfeed-go/types/version"
)

// ============================ Package Variables ============================
var (
	LibVersion    = version.LibVersion
	SchemaVersion = database.SchemaVersionLatest
	ReleaseDate   = version.LibReleaseDate
)

// ============================ Main Entry Point =============================
func main() {
	var config subscriber.Config = *subscriber.DefaultFeederConfig
	var subscriber *subscriber.Subscriber = &subscriber.Subscriber{}
	var feeds []*hyperfeed.Feed
	var timeout, workers int
	var opmlURI, feedHref string
	var ctx context.Context = context.Background()
	var args []string = os.Args[1:]
	var w io.Writer = os.Stdout
	var logfile string
	var err error

	// Parse command line arguments
	if slices.Contains(args, "-h") || slices.Contains(args, "--help") {
		fmt.Println(usage())
		os.Exit(0)
	}
	if config.DatabasePath, err = get(args, "-d", "--database"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if config.DatabasePath == "" {
		config.DatabasePath = "hyperfeed.db"
	}
	if opmlURI, err = get(args, "-o", "--opml"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if feedHref, err = get(args, "-f", "--feed"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if logfile, err = get(args, "--logfile"); len(logfile) > 0 {
		w = &bytes.Buffer{}
	}
	if timeoutString, err := get(args, "--timeout"); err == nil && timeoutString != "" {
		if timeout, err = strconv.Atoi(timeoutString); err != nil {
			fmt.Printf("Error parsing timeout duration %q: %v\n", timeoutString, err)
			os.Exit(1)
		}
	}
	if workersString, err := get(args, "--workers"); err == nil && workersString != "" {
		if workers, err = strconv.Atoi(workersString); err != nil {
			fmt.Printf("Error parsing workers count %q: %v\n", workersString, err)
			os.Exit(1)
		}
	}
	if slices.Contains(args, "parse") {
		if opmlURI == "" {
			opmlURI = commandValue(args, "parse")
		}
		if opmlURI == "" {
			fmt.Fprintln(os.Stderr, "Error parsing OPML: parse requires -o/--opml <uri> or parse <uri>")
			os.Exit(1)
		}
		if err = printParsedFeeds(ctx, opmlURI, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing OPML: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Initialize the Subscriber
	if err = subscriber.Init(ctx, &config); err != nil {
		fmt.Printf("Error initializing subscriber: %v\n", err)
		os.Exit(1)
	}

	// Import feeds from the OPML source (if provided)
	if opmlURI != "" {
		// ImportFeeds persists the subscription list and the resolved feeds
		if feeds, err = subscriber.ImportFeeds(ctx, opmlURI, os.Stdout); err != nil {
			fmt.Printf("Error loading subscriber: %v\n", err)
			os.Exit(1)
		}
	}

	// If a single feed HREF is provided, filter to that feed only
	if feedHref != "" {
		var feed *hyperfeed.Feed = &hyperfeed.Feed{}
		if feed.URL, err = href.Parse(feedHref); err != nil {
			fmt.Printf("Error parsing feed URL %q: %v\n", feedHref, err)
			os.Exit(1)
		}
		feeds = []*hyperfeed.Feed{feed}
	}

	// Export feeds
	if slices.Contains(args, "export") {
		var out []byte
		if out, err = subscriber.Export(ctx, false, os.Stdout); err != nil {
			fmt.Printf("Error exporting subscriber: %v\n", err)
			os.Exit(1)
		} else {
			fmt.Println(string(out))
		}
	}

	// Add feed
	if slices.Contains(args, "add") {
		for _, arg := range args[1:] {
			var u *href.Href
			var feed hyperfeed.Feed
			var err error
			fmt.Printf("feed %q add\n", arg)
			if u, err = href.Parse(arg); err != nil {
				fmt.Printf("feed %q error: %s\n", arg, err)
				os.Exit(1)
			}
			feed = hyperfeed.Feed{URL: u}
			feed.ID = feed.Identifier()
			if err = subscriber.Database.UpsertRow(ctx, nil, &feed); err != nil {
				fmt.Printf("feed %q error: %s\n", arg, err)
				os.Exit(1)
			}
			fmt.Printf("feed %q added\n", arg)
		}
	}

	// Refresh feeds
	if slices.Contains(args, "refresh") {
		if err = subscriber.Refresh(ctx, feeds, timeout, workers, w); err != nil {
			fmt.Printf("Error refreshing subscriber: %v\n", err)
			os.Exit(1)
		}
		if w != os.Stdout {
			var f *os.File
			if f, err = os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err != nil {
				fmt.Printf("Error opening logfile %q: %v\n", logfile, err)
				os.Exit(1)
			}
			defer f.Close()
			if _, err = f.WriteString(w.(*bytes.Buffer).String()); err != nil {
				fmt.Printf("Error writing to logfile %q: %v\n", logfile, err)
				os.Exit(1)
			}
		}
	}
}

// get argument from os.Args by flag name(s); last flag wins
func get(args []string, flags ...string) (value string, err error) {
	for _, f := range flags {
		if slices.Contains(args, f) {
			index := slices.Index(args, f)
			if index+1 >= len(args) {
				return "", fmt.Errorf("missing value for argument %q", f)
			}
			value = args[index+1]
		}
	}
	return value, err
}

func commandValue(args []string, command string) string {
	if !slices.Contains(args, command) {
		return ""
	}
	index := slices.Index(args, command)
	if index+1 >= len(args) {
		return ""
	}
	value := args[index+1]
	if len(value) > 0 && value[0] == '-' {
		return ""
	}
	return value
}

func printParsedFeeds(ctx context.Context, opmlURI string, out io.Writer, logs io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	if logs == nil {
		logs = io.Discard
	}
	subscriptions, err := subscriber.LoadSubscriptionList(ctx, opmlURI)
	if err != nil {
		return err
	}
	feeds, err := subscriptions.InitFeeds(logs)
	if err != nil {
		return err
	}
	for _, feed := range feeds {
		var feedURL, htmlURL string
		if feed.URL != nil {
			feedURL = feed.URL.String()
		}
		if feed.HTMLURL != nil {
			htmlURL = feed.HTMLURL.String()
		}
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\n", feed.Title, feedURL, htmlURL); err != nil {
			return err
		}
	}
	return nil
}

func usage() (usage string) {
	var commands [][2]string = [][2]string{
		{"parse", "Parse OPML and print discovered feeds without writing to the database"},
		{"refresh", "Refresh feeds in the subscriber database"},
	}
	var options [][2]string = [][2]string{
		{"-d, --database <path>", "Path to HyperFeed database file (required)"},
		{"-o, --opml <uri>", "Import feeds from the OPML subscription list at <uri> (a local path, file:// URL, or http(s):// URL)"},
		{"-f, --feed <href>", "Refresh a single feed at the provided HREF"},
		{"    --timeout <seconds>", "Timeout duration for network operations (in seconds)"},
		{"    --workers <count>", "Maximum number of concurrent feed workers (0 = unlimited)"},
		{"    --logfile <path>", "Output progress to logfile instead of stdout"},
		{"-h, --help", "Print this help message"},
	}
	usage += fmt.Sprintf("hyperfeed %s\n\n", LibVersion)
	usage += "Usage:\n\thyperfeed [options]\n\n"
	usage += "Commands:\n\n"
	for _, command := range commands {
		usage += fmt.Sprintf("\t%-24s %s\n", command[0], command[1])
	}
	usage += "\n"
	usage += "Options:\n\n"
	for _, option := range options {
		usage += fmt.Sprintf("\t%-24s %s\n", option[0], option[1])
	}
	return usage
}
