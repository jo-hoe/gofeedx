package gofeedx

import (
	"strings"
	"testing"
	"time"
)

func TestCollectAndMaxTime(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-2 * time.Hour)
	items := []*Item{
		{Created: past},
		{Updated: now},
		{},
	}
	ts := collectItemTimes(items)
	if len(ts) != 2 {
		t.Fatalf("collectItemTimes expected 2, got %d", len(ts))
	}
	m := maxTime(ts...)
	if !m.Equal(now) {
		t.Errorf("maxTime expected latest time %v, got %v", now, m)
	}
	if !maxTime().IsZero() {
		t.Errorf("maxTime with no inputs should be zero time")
	}
}

func TestFeedBuilder_StrictAndLenient(t *testing.T) {
	// Strict: empty title should error
	b := NewFeed("")
	b.AddItem(NewItem("ok")) // add item to avoid "at least one item" masking title issue
	if _, err := b.Build(); err == nil || !strings.Contains(err.Error(), "feed title required") {
		t.Fatalf("strict builder expected title error, got: %v", err)
	}
	// Lenient: allows empty title and no items when no profiles selected
	b2 := NewFeed("").WithLenient()
	f, err := b2.Build()
	if err != nil {
		t.Fatalf("lenient builder should not fail without profiles: %v", err)
	}
	if f == nil || f.Title != "" || len(f.Items) != 0 {
		t.Errorf("lenient builder returned unexpected feed: %#v", f)
	}
}

func TestFeedBuilder_AddItemFilteringAndErrors(t *testing.T) {
	// Add an item that fails its own strict checks -> AddItem() ignores error and stores nil
	b := NewFeed("t")
	b.AddItem(NewItem("")) // strict item requires title or description, so Build() of item returns error -> nil
	_, err := b.Build()
	if err != nil {
		t.Fatalf("Build() unexpected error when item validation fails: %v", err)
	}
}

func TestFeedBuilder_WithSortAndAddItemFunc(t *testing.T) {
	b := NewFeed("t")
	// Add items via AddItemFunc
	b.AddItemFunc(func(ib *ItemBuilder) { ib.WithTitle("b") })
	b.AddItemFunc(func(ib *ItemBuilder) { ib.WithTitle("a") })
	// Sort descending by title
	b.WithSort(func(a, c *Item) bool { return a.Title > c.Title })
	f, err := b.Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	if len(f.Items) != 2 || f.Items[0].Title != "b" || f.Items[1].Title != "a" {
		t.Errorf("expected sorted order [b, a], got [%s, %s]", f.Items[0].Title, f.Items[1].Title)
	}
}

func TestFeedBuilder_AutoIDsAndAtomUpdatedDefault(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-time.Hour)
	b := NewFeed("t").WithLink("https://example.org/").WithAuthor("Author", "").WithProfiles(ProfileAtom, ProfileJSON)
	b.AddItem(NewItem("a").WithCreated(earlier))
	b.AddItem(NewItem("b").WithUpdated(now))
	f, err := b.Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	// Atom Updated default should be max(item times)
	if !f.Updated.Equal(now) {
		t.Errorf("feed.Updated default expected %v, got %v", now, f.Updated)
	}
	// Item IDs should be auto-filled with non-empty values and isPermaLink defaulted to "false"
	for i, it := range f.Items {
		if strings.TrimSpace(it.ID) == "" {
			t.Errorf("item[%d] ID expected to be auto-set", i)
		}
		if it.IsPermaLink != "false" {
			t.Errorf("item[%d] IsPermaLink expected \"false\", got %q", i, it.IsPermaLink)
		}
	}
}

func TestFeedBuilder_ProfileValidationError(t *testing.T) {
	// Missing description is invalid for RSS
	b := NewFeed("t").WithProfiles(ProfileRSS)
	b.WithLink("https://example.org/")
	b.AddItem(NewItem("item"))
	_, err := b.Build()
	if err == nil || !strings.Contains(err.Error(), "rss: channel description required") {
		t.Fatalf("expected RSS description error, got: %v", err)
	}
}

func TestItemBuilder_StrictEnclosureValidation(t *testing.T) {
	ib := NewItem("t")
	ib.WithEnclosure("https://cdn.example.org/x.mp3", 0, "audio/mpeg") // invalid length
	if _, err := ib.Build(); err == nil || !strings.Contains(err.Error(), "enclosure requires url, type and positive length") {
		t.Fatalf("expected enclosure validation error, got: %v", err)
	}
}

func TestBuilderConvenienceRenderersThroughProfiles(t *testing.T) {
	// Build a simple valid feed for all targets using builder API
	now := time.Now().UTC()
	b := NewFeed("Title").
		WithLink("https://example.org/").
		WithDescription("Desc").
		WithLanguage("en-us")
	// Item with minimal valid data
	ib := NewItem("Item").
		WithCreated(now).
		WithEnclosure("https://cdn.example.org/a.mp3", 10, "audio/mpeg")
	b.AddItem(ib)
	// Validate and render RSS
	f, err := b.WithProfiles(ProfileRSS, ProfileJSON).Build()
	if err != nil {
		t.Fatalf("Build() unexpected error on profiles: %v", err)
	}
	if _, err := ToRSS(f); err != nil {
		t.Fatalf("ToRSS unexpected error: %v", err)
	}
	if _, err := ToAtom(&Feed{
		Title:   "A",
		Link:    &Link{Href: "https://example.org/"},
		Created: now,
		Items:   []*Item{{Title: "E", Created: now}},
		Author:  &Author{Name: "X"},
	}); err != nil {
		t.Fatalf("ToAtom unexpected error: %v", err)
	}
	if _, err := ToPSP(&Feed{
		Title:       "P",
		Link:        &Link{Href: "https://example.org/p"},
		Description: "d",
		Language:    "en-us",
		FeedURL:     "https://example.org/podcast.rss",
		Categories:  []*Category{{Text: "Tech"}},
		Items: []*Item{{
			Title:   "Ep",
			ID:      "id",
			Created: now,
			Enclosure: &Enclosure{
				Url:    "https://cdn.example.org/a.mp3",
				Type:   "audio/mpeg",
				Length: 10,
			},
		}},
	}); err != nil {
		t.Fatalf("ToPSP unexpected error: %v", err)
	}
	if _, err := ToJSON(f); err != nil {
		t.Fatalf("ToJSON unexpected error: %v", err)
	}
}

// Moved from builder_more_test.go to maintain 1:1 mapping with builder.go

func TestToAtom_NilFeedError(t *testing.T) {
	if _, err := ToAtom(nil); err == nil || !strings.Contains(err.Error(), "nil feed") {
		t.Fatalf("ToAtom expected nil feed error, got: %v", err)
	}
}

func TestToRSS_NilFeedError(t *testing.T) {
	if _, err := ToRSS(nil); err == nil || !strings.Contains(err.Error(), "nil feed") {
		t.Fatalf("ToRSS expected nil feed error, got: %v", err)
	}
}

func TestToPSP_NilFeedError(t *testing.T) {
	if _, err := ToPSP(nil); err == nil || !strings.Contains(err.Error(), "nil feed") {
		t.Fatalf("ToPSP expected nil feed error, got: %v", err)
	}
}

func TestToJSON_NilFeedError(t *testing.T) {
	if _, err := ToJSON(nil); err == nil || !strings.Contains(err.Error(), "nil feed") {
		t.Fatalf("ToJSON expected nil feed error, got: %v", err)
	}
}
