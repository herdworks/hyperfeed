package hyperfeed

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	hgdiscovery "github.com/herdworks/hypergraph-go/discovery"
	hypergraph "github.com/herdworks/hypergraph-go/types"
	"golang.org/x/sync/errgroup"

	href "github.com/herdworks/hyperfeed-go/types/href"
)

const defaultRecommendationWorkers = 4

// RecommendedFeeds discovers feed URLs for the websites listed in a
// Ghost-compatible /.well-known/recommendations.json document.
func RecommendedFeeds(ctx context.Context, source string, timeout int, workers int, verbose bool, w io.Writer) (feeds []*Feed, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = 10
	}
	recommendations := hgdiscovery.Recommendations(ctx, source, timeout, nil, verbose, writer(w))
	return RecommendedFeedsForURLs(ctx, recommendations, timeout, workers, verbose, w)
}

// RecommendedFeedsForURLs discovers feed URLs for an already-parsed list of
// recommended website URLs.
func RecommendedFeedsForURLs(ctx context.Context, recommendations []string, timeout int, workers int, verbose bool, w io.Writer) (feeds []*Feed, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = 10
	}
	if workers <= 0 {
		workers = defaultRecommendationWorkers
	}
	w = writer(w)
	results := make([][]*Feed, len(recommendations))
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(workers)
	for index, recommendation := range recommendations {
		index, recommendation := index, recommendation
		group.Go(func() error {
			results[index] = discoverRecommendedFeeds(ctx, recommendation, timeout, verbose, w)
			return nil
		})
	}
	if err = group.Wait(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, result := range results {
		for _, feed := range result {
			if feed == nil || feed.URL == nil {
				continue
			}
			if id := feed.Identifier(); seen[id] {
				continue
			} else {
				seen[id] = true
			}
			feeds = append(feeds, feed)
		}
	}
	return feeds, nil
}

// RecommendationsOPML converts recommendations.json feed discoveries into an
// OPML subscription list.
func RecommendationsOPML(ctx context.Context, source string, timeout int, workers int, verbose bool, w io.Writer) (out []byte, err error) {
	feeds, err := RecommendedFeeds(ctx, source, timeout, workers, verbose, w)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	subscriptions := SubscriptionList{
		Title:     "Recommended Feeds",
		CreatedAt: now,
		UpdatedAt: now,
		Feeds:     feeds,
	}
	return subscriptions.MarshalOPML(false)
}

func discoverRecommendedFeeds(ctx context.Context, recommendation string, timeout int, verbose bool, w io.Writer) (feeds []*Feed) {
	recommendation = strings.TrimSpace(recommendation)
	if !webURL(recommendation) {
		return nil
	}

	feedHrefs := []string{}
	document := &hypergraph.Document{Href: recommendation}
	if err := document.Download(ctx, verbose, nil, timeout, w); err != nil {
		if verbose {
			fmt.Fprintf(w, "recommendation %q download error: %v\n", recommendation, err)
		}
	} else {
		hyperlinks := &hypergraph.HyperLinkMetadata{}
		hyperlinks.Scrape(ctx, document, verbose, nil, 0, w)
		feedHrefs = append(feedHrefs, hyperlinks.Feeds...)
	}
	if len(feedHrefs) == 0 {
		if feed := hgdiscovery.Crawl(ctx, recommendation, timeout, nil, verbose, w); feed != "" {
			feedHrefs = append(feedHrefs, feed)
		}
	}

	for _, feedHref := range feedHrefs {
		if feed := recommendedFeed(feedHref, recommendation); feed != nil && !slices.ContainsFunc(feeds, func(existing *Feed) bool {
			return existing != nil && existing.Identifier() == feed.Identifier()
		}) {
			feeds = append(feeds, feed)
		}
	}
	return feeds
}

func recommendedFeed(feedHref string, recommendation string) *Feed {
	feedURL, err := href.Parse(feedHref)
	if err != nil || feedURL == nil || !webURL(feedURL.String()) {
		return nil
	}
	htmlURL, _ := href.Parse(recommendation)
	websiteHref := websiteRoot(recommendation)
	feed := &Feed{
		URL:       feedURL,
		HTMLURL:   htmlURL,
		Title:     recommendationTitle(recommendation),
		Following: false,
		Status:    Status{LastStatusCodes: []int{}},
	}
	feed.ID = feed.Identifier()
	if websiteHref != "" {
		feed.Website = Website{
			Entity: hypergraph.Entity{
				Href:    websiteHref,
				HTMLURL: websiteHref,
			},
			Following: false,
		}
		feed.Website.ID = feed.Website.Identifier()
	}
	return feed
}

func webURL(href string) bool {
	u, err := url.Parse(href)
	return err == nil && u.IsAbs() && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

func websiteRoot(href string) string {
	u, err := url.Parse(href)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Path = "/"
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	return u.String()
}

func recommendationTitle(href string) string {
	u, err := url.Parse(href)
	if err != nil || u.Host == "" {
		return href
	}
	return u.Host
}

func writer(w io.Writer) io.Writer {
	if w == nil {
		return os.Stdout
	}
	return w
}
