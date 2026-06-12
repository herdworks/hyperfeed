package subscriber

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/herdworks/hyperfeed-go/internal/database"
	hyperfeed "github.com/herdworks/hyperfeed-go/types"
)

// Unfollow soft-deletes a feed: it deletes the feed's items (cascading to the
// item_* join tables) and sets feeds.following = 0, in one transaction. This is
// the ONLY item-deleting path — writing following=false through the normal
// upsert does not remove items. Website-level following is the caller's
// responsibility.
//
// Like SavePost, it lives in the subscriber package (not on types.Feed) so it
// can use the database directly without the types -> internal/database import
// cycle.
func Unfollow(ctx context.Context, db *database.Database, feed *hyperfeed.Feed) (err error) {
	if feed == nil || db == nil {
		return errors.Join(hyperfeed.ErrFeed, errors.New("Unfollow requires a feed and database"))
	}
	// Require an identifiable feed: with neither id nor url, Identifier() resolves
	// to the "unknown" sentinel and the statements would silently match nothing,
	// masking a caller bug.
	if feed.ID == "" && (feed.URL == nil || feed.URL.URL() == nil) {
		return errors.Join(hyperfeed.ErrFeed, errors.New("Unfollow requires a feed id or url"))
	}
	statusJSON, err := json.Marshal(hyperfeed.Status{LastStatusCodes: []int{}})
	if err != nil {
		return err
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
	id := feed.Identifier()
	if _, err = tx.ExecContext(ctx, `DELETE FROM items WHERE feed_id = ?;`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE feeds SET following = 0, status = ? WHERE id = ?;`, string(statusJSON), id); err != nil {
		return err
	}
	err = tx.Commit()
	return err
}
