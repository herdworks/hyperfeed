package hyperfeed

// Post is the read/write envelope the app thinks in: an item together with the
// website and feed it belongs to. It exists because an item needs an associated
// website to render the timeline website card. It mirrors the Swift
// HyperFeed.Post and the posts_view shape.
//
// Post is a pure wire + read type: it carries no persistence logic. Reads go
// through posts_view (DatabaseReadable, below); writes go through
// subscriber.SavePost, which fans a Post out into the websites/feeds/items
// (+ bylines/attachments/links + join) tables using the database primitives.
// Save lives in the subscriber package (not here) so it can import
// internal/database directly — types cannot, because internal/database's tests
// import types.
type Post struct {
	Item    *Item    `json:"item"`
	Website *Website `json:"website"`
	Feed    *Feed    `json:"feed"`
}

// ============================ DatabaseRow (read) ============================
// Post is read from posts_view via database.ListRecords[*Post]/ReadRow. Those
// generics constrain on the combined DatabaseRow interface (read + write), so
// the write-side methods below must exist — but a Post is never written through
// the generic upsert (a post spans tables; use subscriber.SavePost). They are
// therefore trivial stubs: inert at runtime, present only to satisfy the
// interface for the read path.

func (*Post) DatabaseObject() string                     { return "post" }
func (*Post) DatabaseView() string                       { return "posts_view" }
func (*Post) DatabaseViewIdentifier() string             { return "id" }
func (*Post) DatabaseTable() string                      { return "posts_view" }
func (*Post) DatabaseTableIdentifier() string            { return "id" }
func (*Post) DatabaseInsertColumns() []string            { return nil }
func (*Post) DatabaseInsertParameters([]string) []string { return nil }
func (*Post) DatabaseInsertValues() []any                { return nil }
func (*Post) DatabaseUpdatesSet([]string) []string       { return nil }

func (post *Post) Hash() string {
	if post == nil || post.Item == nil {
		return ""
	}
	return post.Item.Hash()
}

func (post *Post) Identifier() string {
	if post == nil || post.Item == nil {
		return ""
	}
	return post.Item.Identifier()
}
