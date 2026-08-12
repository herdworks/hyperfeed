package subscriber

import (
	"context"
	"database/sql"
	"errors"

	"github.com/herdworks/hyperfeed"
	"github.com/herdworks/hyperfeed/internal/database"
)

// SavePost persists a one-off post — its website, feed, item, and the item's
// bylines, attachments, and links (+ join rows) — in a single transaction.
//
// It is a free function (not a *Subscriber method) and deliberately does NOT
// reuse the Subscriber.Save* helpers: a one-off post is out-of-band of
// subscription refresh, so it has no Fetched()/interval gate. It lives in the
// subscriber package (rather than on the types.Post type) so it can use the
// internal/database primitives directly — types cannot import internal/database,
// because internal/database's tests import types.
//
// following (like every other column) is written VERBATIM from the in-memory
// post; SavePost does not preserve or recompute follow-state. A Post payload
// must already carry authoritative following values (read-before-write), exactly
// like the normal refresh loads feeds from the DB before re-saving them so
// feed.following round-trips. The only path that builds a Post from a non-DB
// payload is sponsored generation, which hydrates the sponsor's website/feed
// following from the DB before calling SavePost. SavePost is idempotent (stable
// identifiers upsert).
func SavePost(ctx context.Context, db *database.Database, post *hyperfeed.Post) (err error) {
	if post == nil || db == nil {
		return errors.Join(hyperfeed.ErrFeed, errors.New("SavePost requires a post and database"))
	}
	if post.Item == nil || post.Feed == nil || post.Website == nil {
		return errors.Join(hyperfeed.ErrFeed, errors.New("SavePost requires item, feed, and website"))
	}
	// Identifiers are derived from these URLs; guard so a malformed payload fails
	// gracefully instead of panicking (this runs across the cgo boundary).
	if post.Feed.URL == nil || post.Item.URL == nil {
		return errors.Join(hyperfeed.ErrFeed, errors.New("SavePost requires feed.url and item.url"))
	}

	// Wire the relationships the persistence layer derives identifiers from.
	post.Feed.Website = *post.Website
	post.Item.Feed = post.Feed

	// Ensure identifiers are computed before relating rows.
	post.Website.Identifier()
	post.Feed.Identifier()
	post.Item.Identifier()
	for i := range post.Item.Bylines {
		post.Item.Bylines[i].Identifier()
	}
	for i := range post.Item.Attachments {
		post.Item.Attachments[i].Identifier()
	}

	var tx *sql.Tx
	if tx, err = db.BeginTransaction(ctx, nil); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	websiteID := post.Feed.Website.Identifier()
	feedID := post.Feed.Identifier()
	itemID := post.Item.Identifier()

	if err = db.UpsertRow(ctx, tx, post.Website); err != nil {
		return err
	}
	if err = db.UpsertRow(ctx, tx, post.Feed); err != nil {
		return err
	}
	if err = db.UpsertRow(ctx, tx, post.Item); err != nil {
		return err
	}
	for i := range post.Item.Bylines {
		byline := &post.Item.Bylines[i]
		if err = db.UpsertRow(ctx, tx, byline); err != nil {
			return err
		}
		if err = db.RelateRows(ctx, tx, "item_bylines",
			[]string{"website_id", "feed_id", "item_id", "byline_id"},
			[]any{websiteID, feedID, itemID, byline.Identifier()}); err != nil {
			return err
		}
	}
	for i := range post.Item.Attachments {
		attachment := &post.Item.Attachments[i]
		if err = db.InsertRow(ctx, tx, attachment); err != nil {
			return err
		}
		if err = db.RelateRows(ctx, tx, "item_attachments",
			[]string{"website_id", "feed_id", "item_id", "attachment_id", "mimetype", "enclosure", "rank"},
			[]any{websiteID, feedID, itemID, attachment.Identifier(), attachment.Mimetype, attachment.Enclosure, attachment.Rank}); err != nil {
			return err
		}
	}
	for _, link := range post.Item.Links {
		if err = db.InsertRow(ctx, tx, link); err != nil {
			return err
		}
		if err = db.RelateRows(ctx, tx, "item_links",
			[]string{"website_id", "feed_id", "item_id", "link_id", "weight"},
			[]any{websiteID, feedID, itemID, link.Identifier(), link.Weight}); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}
