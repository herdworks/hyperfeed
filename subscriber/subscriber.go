package subscriber

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/herdworks/hyperfeed-go/feeder"
	"github.com/herdworks/hyperfeed-go/internal/database"
	reader "github.com/herdworks/hyperfeed-go/reader"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
	href "github.com/herdworks/hyperfeed-go/types/href"
)

// TYPES
// =============================================================================

// Subscriber manages one or more SubscriptionLists
type Subscriber struct {
	Config   *Config
	Database *database.Database
	Status   *Status
}

// Config holds default Feeder configuration parameters
type Config struct {
	// DatabasePath is the filepath to the database used to persist feed and item state
	DatabasePath string `json:"db_path"`

	// DefaultInterval is the baseline polling interval when no hints are available.
	// Implementations must never schedule requests more frequently than this value.
	DefaultInterval time.Duration `json:"default_interval"`
}

// Status holds Subscriber runtime status information
type Status struct {
	Duration           time.Duration     `json:"duration,omitempty"`
	UpdatedAt          time.Time         `json:"updated_at,omitempty"`
	FeedsCount         hyperfeed.Counter `json:"feeds_count,omitempty"`
	FeedsSkipped       hyperfeed.Counter `json:"feeds_skipped,omitempty"`
	FeedsRateLimited   hyperfeed.Counter `json:"feeds_rate_limited,omitempty"`
	FeedsFetched       hyperfeed.Counter `json:"feeds_fetched,omitempty"`
	FeedsTooSlowJunior hyperfeed.Counter `json:"feeds_too_slow_junior,omitempty"`
	FeedsFailed        hyperfeed.Counter `json:"feeds_failed,omitempty"`
	ItemsFetched       hyperfeed.Counter `json:"items_fetched,omitempty"`
	ItemsDuplicate     hyperfeed.Counter `json:"items_duplicate,omitempty"`
	ItemsAdded         hyperfeed.Counter `json:"items_added,omitempty"`
	ItemsUpdated       hyperfeed.Counter `json:"items_updated,omitempty"`
}

// VARIABLES
// =============================================================================

// errors
var (
	ErrNotImplemented   = errors.New("not implemented")
	ErrReadiness        = errors.New("subscriber not ready")
	ErrSubscriber       = errors.New("subscriber error")
	ErrSubscriptionList = errors.New("subscription list error")
	ErrDatabase         = errors.New("database error")
	ErrScraper          = errors.New("scraper error")
)

// defaults
var (
	DefaultFeederConfig = &Config{
		DefaultInterval: IntervalActivityHigh,
	}
	IntervalActivityLive   = 1 * time.Minute
	IntervalActivityHigh   = 10 * time.Minute
	IntervalActivityMedium = 60 * time.Minute
	IntervalActivityLow    = 240 * time.Minute  // 4 hours
	IntervalActivityDaily  = 1440 * time.Minute // 24 hours
)

// METHODS
// =============================================================================

// Export renders the subscriber's feeds table as an OPML subscription list
// (via hyperfeed.SubscriptionList.MarshalOPML).
func (subscriber *Subscriber) Export(ctx context.Context, redacted bool, w io.Writer) (out []byte, err error) {
	var feeds []*hyperfeed.Feed
	if subscriber.Database == nil {
		return nil, ErrDatabase
	}
	if subscriber.Database.DB == nil {
		if err = subscriber.Database.Open(ctx); err != nil {
			return nil, err
		}
		defer subscriber.Database.Close()
	}
	if err = subscriber.Database.PingContext(ctx); err != nil {
		return nil, err
	}
	if feeds, err = database.ListRecords[*hyperfeed.Feed](ctx, subscriber.Database); err != nil {
		return nil, err
	}
	// Only export followed feeds: this excludes both soft-deleted (unfollowed)
	// feeds and generated feeds (e.g. the sponsored-posts feed), which carry
	// following=0 and must never leak into an OPML backup or share.
	feeds = slices.DeleteFunc(feeds, func(feed *hyperfeed.Feed) bool {
		return feed == nil || !feed.Following
	})
	var subscriptions hyperfeed.SubscriptionList = hyperfeed.SubscriptionList{
		Title:     "HyperFeed Subscriptions",
		CreatedAt: time.Now(),
		Feeds:     feeds,
	}
	return subscriptions.MarshalOPML(redacted)
}

// Backup exposes the Subscriber's database backup functionality
func (subscriber *Subscriber) Backup(ctx context.Context, destinationPath string, w io.Writer) (err error) {
	if subscriber.Database == nil {
		return ErrDatabase
	}
	if subscriber.Database.Path == "" {
		return ErrDatabase
	}
	if w == nil {
		w = os.Stdout
	}
	return subscriber.Database.Backup(ctx, destinationPath, w)
}

// Refresh refreshes all subscribed feeds concurrently
func (subscriber *Subscriber) Refresh(ctx context.Context, feeds []*hyperfeed.Feed, timeout int, workers int, w io.Writer) (err error) {
	var client *http.Client
	var ticker *errgroup.Group
	var cancellation context.Context
	var start time.Time = time.Now().UTC()
	subscriber.Status = &Status{UpdatedAt: start}
	ticker, cancellation = errgroup.WithContext(ctx)
	if workers > 0 {
		ticker.SetLimit(workers)
	}
	if w == nil {
		w = os.Stdout
	}

	// apply timeout
	if timeout <= 60 {
		fmt.Fprintf(w, "client.timeout: %ds\n", timeout)
		client = reader.Client(reader.Config{Timeout: time.Duration(timeout) * time.Second})
	} else {
		fmt.Fprintf(w, "client.timeout: %dms\n", max(500, timeout))
		client = reader.Client(reader.Config{Timeout: time.Duration(max(500, timeout)) * time.Millisecond})
	}

	// check database
	if subscriber.Database == nil || subscriber.Database.DB == nil {
		return ErrDatabase
	}

	// Resolve provided feeds with state from database
	if feeds, err = subscriber.ResolveFeeds(ctx, feeds); err != nil {
		return err
	}

	// If no feeds provided, load all FOLLOWED feeds in the database. Soft-deleted
	// (unfollowed) feeds and generated feeds (e.g. the sponsored-posts feed)
	// carry following=0 and are skipped here so unfollowed feeds are not
	// re-fetched; generated feeds are refreshed via an explicit feed list.
	if len(feeds) == 0 {
		if feeds, err = database.ListRecords[*hyperfeed.Feed](ctx, subscriber.Database); err != nil {
			return err
		}
		feeds = slices.DeleteFunc(feeds, func(feed *hyperfeed.Feed) bool {
			return feed == nil || !feed.Following
		})
	}

	// Refresh all feeds concurrently
	for _, feed := range feeds {
		feed := feed
		ff := &feeder.Feeder{Feed: feed}
		if ff.Feed.Interval == 0 {
			ff.Feed.Interval = 5 * time.Minute // default interval
		}
		ticker.Go(func() (e error) {
			ff.Tick(cancellation, client, start, w) // ignore tick errors; they are recorded in f.Feed.Status
			// Skip the HyperGraph website-metadata scrape for local generated
			// feeds (file:// scheme): their website metadata is authored in the
			// feed file, and scraping would issue a pointless network request and
			// could clobber the authored (e.g. sponsor) card.
			local := ff.Feed.URL != nil && ff.Feed.URL.URL() != nil && ff.Feed.URL.URL().Scheme == "file"
			if !local && ff.Feed.Status.LastStatusCode == 200 && time.Since(ff.Feed.Status.LastEntityUpdateAt) > 24*time.Hour {
				if e = ff.Feed.WebsiteMetadata(ctx, timeout, w); e != nil {
					fmt.Fprintf(w, "graph %q website error: %v\n", ff.Feed.HTMLURL.String(), e)
					ff.Feed.Status.LastEntityError = e.Error()
				}
				ff.Feed.Status.LastEntityUpdateAt = start
			}
			subscriber.Status.Log(ff.Feed, e, w)
			return nil
		})
	}
	if err = ticker.Wait(); err != nil {
		return err
	}
	fmt.Fprintf(w, "fetch complete in %v\n", time.Since(start).Truncate(time.Millisecond))
	if err = subscriber.SaveWebsites(ctx, w, feeds); err != nil {
		return err
	}
	if err = subscriber.SaveFeeds(ctx, w, feeds); err != nil {
		return err
	}
	if err = subscriber.SaveItems(ctx, w, feeds); err != nil {
		return err
	} else {
		// only count items if we successfully saved them
		if a, u, err := subscriber.RefreshedItemCounts(ctx, w, start); err != nil {
			fmt.Fprintf(w, "error counting refreshed items: %v\n", err)
		} else {
			subscriber.Status.ItemsAdded.Increment(a)
			subscriber.Status.ItemsUpdated.Increment(u)
		}
	}
	if err = subscriber.SaveBylines(ctx, w, feeds); err != nil {
		return err
	}
	if err = subscriber.SaveBylineRelations(ctx, w, feeds); err != nil {
		return err
	}
	if err = subscriber.SaveLinks(ctx, w, feeds); err != nil {
		return err
	}
	if err = subscriber.SaveLinkRelations(ctx, w, feeds); err != nil {
		return err
	}
	if err = subscriber.SaveAttachments(ctx, w, feeds); err != nil {
		return err
	}
	if err = subscriber.SaveAttachmentRelations(ctx, w, feeds); err != nil {
		return err
	}
	subscriber.Status.Duration = time.Since(start)
	fmt.Fprintln(w, subscriber.Status.Print())
	return nil
}

// Ready checks if the Subscriber is properly initialized
func (subscriber *Subscriber) Ready(ctx context.Context) (ready bool, err error) {
	if subscriber.Config == nil {
		return false, errors.Join(ErrReadiness, errors.New("subscriber configuration is nil"))
	}
	if subscriber.Database == nil {
		return false, errors.Join(ErrReadiness, errors.New("subscriber database is nil"))
	}
	if err = subscriber.Database.PingContext(ctx); err != nil {
		return false, errors.Join(ErrReadiness, fmt.Errorf("subscriber database %q is unreachable", subscriber.Database.Path), err)
	}
	return true, nil
}

// Init initializes the Subscriber database
func (subscriber *Subscriber) Init(ctx context.Context, config *Config) (err error) {
	if subscriber.Database == nil {
		subscriber.Database = &database.Database{} // init empty database
	}
	if subscriber.Config == nil {
		subscriber.Config = config // use provided config
	}
	if subscriber.Config == nil {
		cc := *DefaultFeederConfig // copy of default config
		subscriber.Config = &cc    // init subscriber.Config with defaults
	}
	if subscriber.Config.DatabasePath == "" {
		if pwd, err := os.Getwd(); err == nil {
			subscriber.Config.DatabasePath = filepath.Join(pwd, "hyperfeed.db") // default to current working directory
		}
	}
	subscriber.Database.Path = filepath.Clean(subscriber.Config.DatabasePath)

	// Open the database, creating it if necessary
	if err = subscriber.Database.Open(ctx); err != nil {
		return errors.Join(ErrSubscriber, fmt.Errorf("subscriber database %q open failed", subscriber.Database.Path), err)
	}
	if err = subscriber.Database.Initialize(ctx); err != nil {
		return errors.Join(ErrSubscriber, fmt.Errorf("subscriber database %q initialize failed", subscriber.Database.Path), err)
	}
	return nil
}

// Open opens the Subscriber database without initializing it (no schema
// creation, migration, or maintenance). Use Init for the canonical
// create+migrate entrypoint; Open is for callers that only need an open
// connection (e.g. refresh) and expect migrations to have already run.
func (subscriber *Subscriber) Open(ctx context.Context, config *Config) (err error) {
	if subscriber.Database == nil {
		subscriber.Database = &database.Database{} // init empty database
	}
	if subscriber.Config == nil {
		subscriber.Config = config // use provided config
	}
	if subscriber.Config == nil {
		cc := *DefaultFeederConfig // copy of default config
		subscriber.Config = &cc    // init subscriber.Config with defaults
	}
	if subscriber.Config.DatabasePath == "" {
		if pwd, err := os.Getwd(); err == nil {
			subscriber.Config.DatabasePath = filepath.Join(pwd, "hyperfeed.db") // default to current working directory
		}
	}
	subscriber.Database.Path = filepath.Clean(subscriber.Config.DatabasePath)

	// Open the database, creating it if necessary
	if err = subscriber.Database.Open(ctx); err != nil {
		return errors.Join(ErrSubscriber, fmt.Errorf("subscriber database %q open failed", subscriber.Database.Path), err)
	}
	return nil
}

// ImportFeeds reads an OPML subscription list from uri, persists it, then
// initializes, resolves, and saves its feeds to the feeds table. This is the
// import-and-persist pipeline; use PreviewFeeds to resolve without writing.
func (subscriber *Subscriber) ImportFeeds(ctx context.Context, uri string, w io.Writer) (feeds []*hyperfeed.Feed, err error) {
	var subscriptions *hyperfeed.SubscriptionList
	if w == nil {
		w = os.Stdout
	}
	if ready, err := subscriber.Ready(ctx); !ready || err != nil {
		return nil, err
	}
	if subscriptions, err = LoadSubscriptionList(ctx, uri); err != nil {
		return nil, err
	}
	if err = subscriber.SaveSubscriptionList(ctx, subscriptions); err != nil {
		return nil, err
	}
	if feeds, err = subscriptions.InitFeeds(w); err != nil {
		return nil, err
	}
	if feeds, err = subscriber.ResolveFeeds(ctx, feeds); err != nil {
		return nil, err
	}
	if err = subscriber.SaveFeeds(ctx, w, feeds); err != nil {
		return nil, err
	}
	return feeds, nil
}

// PreviewFeeds reads an OPML subscription list from uri and returns its
// initialized and resolved feeds without persisting anything — neither the
// subscription list nor the feeds are written to the database. Use ImportFeeds
// to persist.
func (subscriber *Subscriber) PreviewFeeds(ctx context.Context, uri string, w io.Writer) (feeds []*hyperfeed.Feed, err error) {
	var subscriptions *hyperfeed.SubscriptionList
	if w == nil {
		w = os.Stdout
	}
	if ready, err := subscriber.Ready(ctx); !ready || err != nil {
		return nil, err
	}
	if subscriptions, err = LoadSubscriptionList(ctx, uri); err != nil {
		return nil, err
	}
	if feeds, err = subscriptions.InitFeeds(w); err != nil {
		return nil, err
	}
	if feeds, err = subscriber.ResolveFeeds(ctx, feeds); err != nil {
		return nil, err
	}
	return feeds, nil
}

// ResolveFeeds loads existing feed state from the database (if any)
//
// NOTE: The only required field to resolve a given feed is the feed.URL, which is used by feed.Identifier() to look up existing records.
func (subscriber *Subscriber) ResolveFeeds(ctx context.Context, feeds []*hyperfeed.Feed) ([]*hyperfeed.Feed, error) {
	for _, feed := range feeds {
		if err := subscriber.Database.ReadRow(ctx, feed); err != nil {
			return nil, err
		}
	}
	return feeds, nil
}

// SaveSubscriptionList persists the SubscriptionList to the database
func (subscriber *Subscriber) SaveSubscriptionList(ctx context.Context, subscriptions *hyperfeed.SubscriptionList) (err error) {
	if err = subscriber.Database.PingContext(ctx); err != nil {
		return errors.Join(ErrDatabase, errors.New("pinging database"), err)
	}
	if err = subscriber.Database.UpsertRow(ctx, nil, subscriptions); err != nil {
		return errors.Join(ErrDatabase, errors.New("upserting subscription list"), err)
	}
	return nil
}

func (subscriber *Subscriber) SaveWebsites(ctx context.Context, w io.Writer, feeds []*hyperfeed.Feed) (err error) {
	var tx *sql.Tx
	var count map[string]int = map[string]int{}
	if tx, err = subscriber.Database.BeginTransaction(ctx, nil); err != nil {
		return errors.Join(ErrDatabase, errors.New("starting transaction"), err)
	}
	defer subscriber.Database.EndTransaction(tx, err)
	for _, feed := range feeds {
		// A followed feed's website must be followed (so a newly-created website
		// appears in the Following view; the parser reconstructs feed.Website each
		// refresh without `following`). Only UPGRADE — never downgrade — because a
		// website is followed iff it has ≥1 followed feed, and websites are shared
		// (WebsiteIdentifier collapses to the root domain). Downgrades happen only
		// via the explicit unfollow path, not as a side effect of saving a sibling
		// feed that happens to be unfollowed.
		if feed.Following {
			feed.Website.Following = true
		}
		if err = subscriber.Database.UpsertRow(ctx, tx, &feed.Website); err != nil {
			return errors.Join(ErrDatabase, errors.New("upserting website"), err)
		}
		count[feed.Website.Identifier()] += 1
	}
	fmt.Fprintf(w, "websites written: %d\n", len(slices.Collect(maps.Keys(count))))
	return tx.Commit()
}
func (subscriber *Subscriber) SaveFeeds(ctx context.Context, w io.Writer, feeds []*hyperfeed.Feed) (err error) {
	var tx *sql.Tx
	var count int = 0
	if tx, err = subscriber.Database.BeginTransaction(ctx, nil); err != nil {
		return errors.Join(ErrDatabase, errors.New("starting transaction"), err)
	}
	defer subscriber.Database.EndTransaction(tx, err)
	for _, feed := range feeds {
		if err = subscriber.Database.UpsertRow(ctx, tx, feed); err != nil {
			return errors.Join(ErrDatabase, errors.New("upserting feed"), err)
		}
		count++
	}
	fmt.Fprintf(w, "feeds written: %d\n", count)
	return tx.Commit()
}
func (subscriber *Subscriber) SaveItems(ctx context.Context, w io.Writer, feeds []*hyperfeed.Feed) (err error) {
	var transaction *sql.Tx
	var count int = 0
	for _, feed := range feeds {
		if !feed.Status.Fetched() || feed.Status.Skipped() {
			continue
		}
		if transaction, err = subscriber.Database.BeginTransaction(ctx, nil); err != nil {
			return errors.Join(ErrDatabase, errors.New("starting transaction"), err)
		}
		defer subscriber.Database.EndTransaction(transaction, err)
		for _, item := range feed.Items {
			if err = subscriber.Database.UpsertRow(ctx, transaction, item); err != nil {
				return errors.Join(ErrDatabase, errors.New("upserting item"), err)
			}
			count++
		}
		if err = transaction.Commit(); err != nil {
			return errors.Join(ErrDatabase, errors.New("upserting item"), err)
		}
		if dups := feed.DuplicateItems(); len(dups) > 0 {
			fmt.Printf("feed %q has %d duplicate items\n", feed.URL, len(dups))
			for _, d := range dups {
				fmt.Printf("\t%s: %s\n", d.Identifier(), d.URL)
			}
			subscriber.Status.ItemsDuplicate.Increment(len(dups))
		}
	}
	fmt.Fprintf(w, "items written: %d\n", count)
	return nil
}
func (subscriber *Subscriber) SaveBylines(ctx context.Context, w io.Writer, feeds []*hyperfeed.Feed) (err error) {
	var transaction *sql.Tx
	var count int = 0
	for _, feed := range feeds {
		if !feed.Status.Fetched() || feed.Status.Skipped() {
			continue
		}
		if transaction, err = subscriber.Database.BeginTransaction(ctx, nil); err != nil {
			return errors.Join(ErrDatabase, errors.New("starting transaction"), err)
		}
		defer subscriber.Database.EndTransaction(transaction, err)
		for _, item := range feed.Items {
			for _, byline := range item.Bylines {
				if err = subscriber.Database.UpsertRow(ctx, transaction, &byline); err != nil {
					return errors.Join(ErrDatabase, errors.New("upserting byline"), err)
				}
				count++
			}
		}
		if err = transaction.Commit(); err != nil {
			return errors.Join(ErrDatabase, errors.New("upserting byline"), err)
		}
	}
	fmt.Fprintf(w, "bylines written: %d\n", count)
	return nil
}
func (subscriber *Subscriber) SaveBylineRelations(ctx context.Context, w io.Writer, feeds []*hyperfeed.Feed) (err error) {
	var transaction *sql.Tx
	var count int = 0
	for _, feed := range feeds {
		if !feed.Status.Fetched() || feed.Status.Skipped() {
			continue
		}
		if transaction, err = subscriber.Database.BeginTransaction(ctx, nil); err != nil {
			return errors.Join(ErrDatabase, errors.New("starting transaction"), err)
		}
		defer subscriber.Database.EndTransaction(transaction, err)
		for _, item := range feed.Items {
			for _, byline := range item.Bylines {
				columns := []string{"website_id", "feed_id", "item_id", "byline_id"}
				values := []any{item.Feed.Website.Identifier(), item.Feed.Identifier(), item.Identifier(), byline.Identifier()}
				if err = subscriber.Database.RelateRows(ctx, transaction, "item_bylines", columns, values); err != nil {
					return errors.Join(ErrDatabase, errors.New("inserting item-byline relation"), err)
				}
				count++
			}
		}
		if err = transaction.Commit(); err != nil {
			return errors.Join(ErrDatabase, errors.New("inserting item-byline relation"), err)
		}
	}
	fmt.Fprintf(w, "byline relations written: %d\n", count)
	return nil
}
func (subscriber *Subscriber) SaveLinks(ctx context.Context, w io.Writer, feeds []*hyperfeed.Feed) (err error) {
	var transaction *sql.Tx
	var count int = 0
	for _, feed := range feeds {
		if !feed.Status.Fetched() || feed.Status.Skipped() {
			continue
		}
		if transaction, err = subscriber.Database.BeginTransaction(ctx, nil); err != nil {
			return errors.Join(ErrDatabase, errors.New("starting transaction"), err)
		}
		defer subscriber.Database.EndTransaction(transaction, err)
		for _, item := range feed.Items {
			for _, link := range item.Links {
				if err = subscriber.Database.InsertRow(ctx, transaction, link); err != nil {
					return errors.Join(ErrDatabase, errors.New("inserting link"), err)
				}
				count++
			}
		}
		if err = transaction.Commit(); err != nil {
			return errors.Join(ErrDatabase, errors.New("inserting link"), err)
		}
	}
	fmt.Fprintf(w, "links written: %d\n", count)
	return nil
}
func (subscriber *Subscriber) SaveLinkRelations(ctx context.Context, w io.Writer, feeds []*hyperfeed.Feed) (err error) {
	var transaction *sql.Tx
	var count int = 0
	for _, feed := range feeds {
		if !feed.Status.Fetched() || feed.Status.Skipped() {
			continue
		}
		if transaction, err = subscriber.Database.BeginTransaction(ctx, nil); err != nil {
			return errors.Join(ErrDatabase, errors.New("starting transaction"), err)
		}
		defer subscriber.Database.EndTransaction(transaction, err)
		for _, item := range feed.Items {
			for _, link := range item.Links {
				columns := []string{"website_id", "feed_id", "item_id", "link_id", "weight"}
				values := []any{item.Feed.Website.Identifier(), item.Feed.Identifier(), item.Identifier(), link.Identifier(), link.Weight}
				if err = subscriber.Database.RelateRows(ctx, transaction, "item_links", columns, values); err != nil {
					fmt.Printf("link: %+v\n", link)
					return errors.Join(ErrDatabase, errors.New("inserting item-link relation"), err)
				}
				count++
			}
		}
		if err = transaction.Commit(); err != nil {
			return errors.Join(ErrDatabase, errors.New("inserting item-link relation"), err)
		}
	}
	fmt.Fprintf(w, "link relations written: %d\n", count)
	return nil
}

func (subscriber *Subscriber) SaveAttachments(ctx context.Context, w io.Writer, feeds []*hyperfeed.Feed) (err error) {
	var transaction *sql.Tx
	var count int = 0
	for _, feed := range feeds {
		if !feed.Status.Fetched() || feed.Status.Skipped() {
			continue
		}
		if transaction, err = subscriber.Database.BeginTransaction(ctx, nil); err != nil {
			return errors.Join(ErrDatabase, errors.New("starting transaction"), err)
		}
		defer subscriber.Database.EndTransaction(transaction, err)
		for _, item := range feed.Items {
			for _, attachment := range item.Attachments {
				if err = subscriber.Database.InsertRow(ctx, transaction, &attachment); err != nil {
					return errors.Join(ErrDatabase, errors.New("inserting attachment"), err)
				}
				count++
			}
		}
		if err = transaction.Commit(); err != nil {
			return errors.Join(ErrDatabase, errors.New("inserting attachment"), err)
		}
	}
	fmt.Fprintf(w, "attachments written: %d\n", count)
	return nil
}

func (subscriber *Subscriber) SaveAttachmentRelations(ctx context.Context, w io.Writer, feeds []*hyperfeed.Feed) (err error) {
	var transaction *sql.Tx
	var count int = 0
	for _, feed := range feeds {
		if !feed.Status.Fetched() || feed.Status.Skipped() {
			continue
		}
		if transaction, err = subscriber.Database.BeginTransaction(ctx, nil); err != nil {
			return errors.Join(ErrDatabase, errors.New("starting transaction"), err)
		}
		defer subscriber.Database.EndTransaction(transaction, err)
		for _, item := range feed.Items {
			for _, attachment := range item.Attachments {
				columns := []string{"website_id", "feed_id", "item_id", "attachment_id", "mimetype", "enclosure", "rank"}
				values := []any{item.Feed.Website.Identifier(), item.Feed.Identifier(), item.Identifier(), attachment.Identifier(), attachment.Mimetype, attachment.Enclosure, attachment.Rank}
				if err = subscriber.Database.RelateRows(ctx, transaction, "item_attachments", columns, values); err != nil {
					return errors.Join(ErrDatabase, errors.New("inserting item-attachment relation"), err)
				}
				count++
			}
		}
		if err = transaction.Commit(); err != nil {
			return errors.Join(ErrDatabase, errors.New("inserting item-attachment relation"), err)
		}
	}
	fmt.Fprintf(w, "byline relations written: %d\n", count)
	return nil
}

func (subscriber *Subscriber) RefreshedItemCounts(ctx context.Context, w io.Writer, since time.Time) (int, int, error) {
	var addedCount int = 0
	var addedQuery string = `SELECT COUNT(*) FROM items WHERE added_at = updated_at AND added_at >= ?;`
	var updatedCount int = 0
	var updatedQuery string = `SELECT COUNT(*) FROM items WHERE added_at != updated_at AND updated_at >= ?;`
	var args []any = []any{since.UTC().Format(time.RFC3339)}
	var err error
	if w == nil {
		w = os.Stdout
	}
	if err = subscriber.Database.QueryRowContext(ctx, addedQuery, args...).Scan(&addedCount); err != nil {
		return 0, 0, errors.Join(ErrDatabase, errors.New("counting new items"), err)
	}
	if err = subscriber.Database.QueryRowContext(ctx, updatedQuery, args...).Scan(&updatedCount); err != nil {
		return 0, 0, errors.Join(ErrDatabase, errors.New("counting updated items"), err)
	}
	return addedCount, updatedCount, nil
}

// ============================ Status Methods =================================

func (status *Status) Log(feed *hyperfeed.Feed, _ error, _ io.Writer) {
	status.FeedsCount.Increment(1)
	if feed.Status.Fetched() {
		status.FeedsFetched.Increment(1)
		status.ItemsFetched.Increment(len(feed.Items))
	} else if feed.Status.TimedOut() {
		status.FeedsTooSlowJunior.Increment(1)
	} else if feed.Status.RateLimited() {
		status.FeedsRateLimited.Increment(1)
	} else if feed.Status.Skipped() || !feed.Status.Healthy() {
		status.FeedsSkipped.Increment(1)
	} else if feed.Status.Failed() {
		status.FeedsFailed.Increment(1)
	}
}

func (status *Status) Print() string {
	var report string = "\n"
	report += fmt.Sprintf("Feeds Total:       %6d\n", status.FeedsCount.Count)
	report += fmt.Sprintf("Feeds Skipped:     %6d\n", status.FeedsSkipped.Count)
	report += fmt.Sprintf("Feeds RateLimited: %6d\n", status.FeedsRateLimited.Count)
	report += fmt.Sprintf("Feeds Fetched:     %6d\n", status.FeedsFetched.Count)
	report += fmt.Sprintf("Feeds TooSlow:     %6d\n", status.FeedsTooSlowJunior.Count)
	report += fmt.Sprintf("Feeds Failed:      %6d\n", status.FeedsFailed.Count)
	report += fmt.Sprintf("Items Fetched:     %6d\n", status.ItemsFetched.Count)
	report += fmt.Sprintf("Items Updated:     %6d\n", status.ItemsUpdated.Count)
	report += fmt.Sprintf("Items Added:       %6d\n", status.ItemsAdded.Count)
	report += "-------------------------\n"
	report += fmt.Sprintf("TOTAL:            %7v\n", status.Duration.Truncate(time.Millisecond))
	return report
}

// Status JSON marshal method to convert time.duration to string
func (status *Status) MarshalJSON() (jsonBytes []byte, err error) {
	type StatusJSON Status
	s := &struct {
		Duration string `json:"duration,omitempty"`
		*StatusJSON
	}{
		Duration:   status.Duration.String(),
		StatusJSON: (*StatusJSON)(status),
	}
	return json.Marshal(s)
}

// FUNCTIONS
// =============================================================================

// LoadSubscriptionList reads OPML bytes from uri (a local path,
// file:// URL, or http(s):// URL; dispatched by scheme) and parses them into
// a SubscriptionList.
func LoadSubscriptionList(ctx context.Context, uri string) (subscriptions *hyperfeed.SubscriptionList, err error) {
	var u *href.Href
	var data []byte
	var config reader.Config = reader.Config{
		Timeout:          30 * time.Second,
		MaxContentLength: 16 << 20, // 16 MiB
	}
	if u, err = reader.Normalize(uri); err != nil {
		return nil, err
	}
	// The normalized URI is the canonical location: the subscription list's
	// derived identifier (normalized.Identifier of URI.String()) hashes the
	// normalized form (e.g. bare paths become absolute file:// URLs).
	subscriptions = &hyperfeed.SubscriptionList{URI: u}
	if data, err = reader.Read(ctx, reader.Client(config), u, config); err != nil {
		return nil, err
	}
	if err = subscriptions.Parse(data); err != nil {
		return nil, err
	}
	return subscriptions, nil
}
