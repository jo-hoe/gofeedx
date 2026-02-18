package gofeedx

import (
	"sort"
	"time"
)

// Link represents a related link (generic).
// Only Href is retained as it is used by multiple targets (JSON/RSS/PSP/Atom).
type Link struct {
	Href string
}

// Author represents a person with a name and optional email.
type Author struct {
	Name  string
	Email string
}

// Category represents a generic top-level category.
// Atom/RSS writers use only the first top-level category.
// PSP maps categories to itunes:category (single level).
type Category struct {
	Text string
}

// Image represents a channel-level image.
// Width/Height are omitted from generic fields as they are only used by RSS.
type Image struct {
	Url   string
	Title string
	Link  string
}

// Enclosure represents a media attachment for an item.
// For RSS 2.0 the length attribute is required and should be bytes.
type Enclosure struct {
	Url    string
	Length int64
	Type   string
}

// Item represents a single entry/post/episode.
type Item struct {
	Title       string
	Link        *Link
	Source      *Link
	Author      *Author
	Description string // description in RSS, summary in Atom, summary in JSON
	ID          string // guid in RSS, id in Atom/JSON
	IsPermaLink string // optional parameter for guid in RSS/PSP
	Updated     time.Time
	Created     time.Time
	Enclosure   *Enclosure
	Content     string // HTML content (RSS content:encoded, Atom content, JSON content_html)

	// Extensions holds arbitrary extension nodes to append in item/entry scope (RSS/PSP/Atom) and to be flattened for JSON.
	Extensions []ExtensionNode

	// Generic item fields used by multiple targets
	DurationSeconds int // used by JSON (attachments) and PSP (itunes:duration)
}

// Feed represents a feed/channel across formats.
type Feed struct {
	Title       string
	Link        *Link
	Description string
	Author      *Author
	Updated     time.Time
	Created     time.Time
	ID          string
	Items       []*Item
	Copyright   string
	Image       *Image
	Language    string

	// Extensions holds arbitrary extension nodes to append in channel/feed scope (RSS/PSP/Atom) and to be flattened for JSON.
	Extensions []ExtensionNode

	// Generic channel fields used by multiple targets
	FeedURL    string        // used by JSON (feed_url) and PSP (atom:link rel=self)
	Categories []*Category   // used by RSS/Atom/PSP
}

// Add appends a new item to the feed.
func (f *Feed) Add(item *Item) {
	f.Items = append(f.Items, item)
}

// Sort sorts the Items in the feed with the given less function.
func (f *Feed) Sort(less func(a, b *Item) bool) {
	lessFunc := func(i, j int) bool {
		return less(f.Items[i], f.Items[j])
	}
	sort.SliceStable(f.Items, lessFunc)
}

// anyTimeFormat returns the first non-zero time formatted as a string or "".
func anyTimeFormat(format string, times ...time.Time) string {
	for _, t := range times {
		if !t.IsZero() {
			return t.Format(format)
		}
	}
	return ""
}