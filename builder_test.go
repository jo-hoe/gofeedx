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

// Additional tests to increase coverage

func TestFeedBuilder_WithSortBy_Variants(t *testing.T) {
	now := time.Now().UTC()
	it1 := &Item{Title: "b", ID: "id2", Created: now, Updated: now.Add(5 * time.Minute), DurationSeconds: 5, Author: &Author{Name: "Bob"}}
	it2 := &Item{Title: "a", ID: "id1", Created: now.Add(-1 * time.Hour), Updated: now.Add(-10 * time.Minute), DurationSeconds: 10, Author: &Author{Name: "Alice"}}
	it3 := &Item{Title: "c", ID: "id3", Created: now.Add(2 * time.Hour), Updated: now.Add(1 * time.Hour), DurationSeconds: 2, Author: &Author{Name: "Charlie"}}

	cases := []struct {
		field ItemSortField
		dir   SortDir
		first string
		last  string
	}{
		{SortByTitle, SortAsc, "a", "c"},
		{SortByTitle, SortDesc, "c", "a"},
		{SortByID, SortAsc, "id1", "id3"},
		{SortByID, SortDesc, "id3", "id1"},
		{SortByCreated, SortAsc, it2.Title, it3.Title},   // earliest created first
		{SortByCreated, SortDesc, it3.Title, it2.Title},  // latest created first
		{SortByUpdated, SortAsc, it2.Title, it3.Title},   // earliest updated first
		{SortByUpdated, SortDesc, it3.Title, it2.Title},  // latest updated first
		{SortByDuration, SortAsc, it3.Title, it2.Title},  // shortest first
		{SortByDuration, SortDesc, it2.Title, it3.Title}, // longest first
		{SortByAuthorName, SortAsc, "Alice", "Charlie"},
		{SortByAuthorName, SortDesc, "Charlie", "Alice"},
	}

	for _, c := range cases {
		b := NewFeed("t")
		// use AddItem on builders to ensure same code path
		b.AddItem(&ItemBuilder{item: *it1, strict: false})
		b.AddItem(&ItemBuilder{item: *it2, strict: false})
		b.AddItem(&ItemBuilder{item: *it3, strict: false})
		b.WithSortBy(c.field, c.dir)
		f, err := b.Build()
		if err != nil {
			t.Fatalf("Build error: %v", err)
		}
		if len(f.Items) != 3 {
			t.Fatalf("expected 3 items")
		}
		// Validate based on field expectations
		switch c.field {
		case SortByTitle, SortByDuration, SortByCreated, SortByUpdated:
			if f.Items[0].Title != c.first || f.Items[2].Title != c.last {
				t.Errorf("field %v dir %v: first=%q last=%q", c.field, c.dir, f.Items[0].Title, f.Items[2].Title)
			}
		case SortByID:
			if f.Items[0].ID != c.first || f.Items[2].ID != c.last {
				t.Errorf("field %v dir %v: firstID=%q lastID=%q", c.field, c.dir, f.Items[0].ID, f.Items[2].ID)
			}
		case SortByAuthorName:
			if getAuthorName(f.Items[0].Author) != c.first || getAuthorName(f.Items[2].Author) != c.last {
				t.Errorf("field %v dir %v: firstAuthor=%q lastAuthor=%q", c.field, c.dir, getAuthorName(f.Items[0].Author), getAuthorName(f.Items[2].Author))
			}
		}
	}
}

func TestItemBuilder_WithGUID_Normalization(t *testing.T) {
	ib := NewItem("t").WithGUID("id", "true")
	it, err := ib.Build()
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if it.IsPermaLink != "true" {
		t.Errorf("expected isPermaLink 'true', got %q", it.IsPermaLink)
	}
	ib2 := NewItem("t").WithGUID("id", "TRUE")
	it2, _ := ib2.Build()
	if it2.IsPermaLink != "TRUE" {
		t.Errorf("expected isPermaLink preserved case 'TRUE', got %q", it2.IsPermaLink)
	}
	ib3 := NewItem("t").WithGUID("id", "maybe")
	it3, _ := ib3.Build()
	if it3.IsPermaLink != "maybe" {
		t.Errorf("expected isPermaLink raw 'maybe', got %q", it3.IsPermaLink)
	}
}

func TestWithXMLCDATA_FeedAndItemExtensions(t *testing.T) {
	// Feed-level
	b := NewFeed("t").WithXMLCDATA(false)
	f, err := b.Build()
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	found := false
	for _, n := range f.Extensions {
		if strings.EqualFold(strings.TrimSpace(n.Name), "_xml:cdata") && strings.TrimSpace(n.Text) == "false" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected feed extension _xml:cdata=false")
	}
	// Item-level
	ib := NewItem("it").WithXMLCDATA(false)
	item, err := ib.Build()
	if err != nil {
		t.Fatalf("item Build error: %v", err)
	}
	foundItem := false
	for _, n := range item.Extensions {
		if strings.EqualFold(strings.TrimSpace(n.Name), "_xml:cdata") && strings.TrimSpace(n.Text) == "false" {
			foundItem = true
		}
	}
	if !foundItem {
		t.Errorf("expected item extension _xml:cdata=false")
	}
}

func TestBuilderHelpers_ContainsAndCopy(t *testing.T) {
	set := []Profile{ProfileRSS, ProfileJSON}
	if containsProfile(set, ProfileAtom) {
		t.Errorf("containsProfile unexpected true")
	}
	if !containsAnyProfile(set, ProfileAtom, ProfileJSON) {
		t.Errorf("containsAnyProfile expected true")
	}
	items := []*Item{{Title: "a"}, nil, {Title: "b"}}
	out := copyNonNilItems(items)
	if len(out) != 2 || out[0].Title != "a" || out[1].Title != "b" {
		t.Errorf("copyNonNilItems unexpected result: %+v", out)
	}
}

func TestBuilderHelpers_GettersAndComparators(t *testing.T) {
	// Getters
	if getLinkHref(nil) != "" || getAuthorName(nil) != "" || getAuthorEmail(nil) != "" {
		t.Errorf("nil getters should return empty")
	}
	if getEnclosureLength(nil) != 0 || getEnclosureType(nil) != "" || getEnclosureURL(nil) != "" {
		t.Errorf("nil enclosure getters should return zero/empty")
	}
	if getLinkHref(&Link{Href: "x"}) != "x" || getAuthorName(&Author{Name: "n"}) != "n" || getAuthorEmail(&Author{Email: "e"}) != "e" {
		t.Errorf("getter values incorrect")
	}
	if getEnclosureLength(&Enclosure{Length: 7}) != 7 || getEnclosureType(&Enclosure{Type: "t"}) != "t" || getEnclosureURL(&Enclosure{Url: "u"}) != "u" {
		t.Errorf("enclosure getter values incorrect")
	}

	// Comparators
	if !stringLessCI("a", "b", true) || stringLessCI("a", "b", false) {
		t.Errorf("stringLessCI asc/desc mismatch")
	}
	if !int64Less(1, 2, true) || int64Less(1, 2, false) {
		t.Errorf("int64Less asc/desc mismatch")
	}
	t1 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	if !timeLess(t1, t2, true) || timeLess(t1, t2, false) {
		t.Errorf("timeLess asc/desc mismatch")
	}
	// equality should return false in both dirs
	if timeLess(t1, t1, true) || timeLess(t1, t1, false) {
		t.Errorf("timeLess equals should be false")
	}
}

func TestEnsureItemIDs_Defaults(t *testing.T) {
	items := []*Item{
		{Title: "x", ID: "", IsPermaLink: ""},
		{Title: "y", ID: "set", IsPermaLink: "true"},
	}
	ensureItemIDs(items)
	if !strings.HasPrefix(items[0].ID, "urn:uuid:") {
		t.Errorf("ensureItemIDs should set uuid urn when no data available, got %q", items[0].ID)
	}
	if items[0].IsPermaLink != "false" {
		t.Errorf("ensureItemIDs should default isPermaLink to false, got %q", items[0].IsPermaLink)
	}
	if items[1].ID != "set" || items[1].IsPermaLink != "true" {
		t.Errorf("ensureItemIDs should not change preset values, got %+v", items[1])
	}
}

func TestCDATAHelpers_AndOverrides(t *testing.T) {
	// UnwrapCDATA idempotency
	if UnwrapCDATA("") != "" {
		t.Errorf("UnwrapCDATA empty should return empty")
	}
	if UnwrapCDATA("x") != "x" {
		t.Errorf("UnwrapCDATA non-wrapped should return input")
	}
	if UnwrapCDATA("<![CDATA[x]]>") != "x" {
		t.Errorf("UnwrapCDATA should unwrap single wrapper")
	}
	// needsCDATA
	if needsCDATA("") {
		t.Errorf("needsCDATA empty should be false")
	}
	if !needsCDATA("<p>x</p>") || !needsCDATA("a & b") {
		t.Errorf("needsCDATA should be true for html or ampersand")
	}
	// UseCDATAFromExtensions defaults and overrides
	if !UseCDATAFromExtensions(nil) {
		t.Errorf("default CDATA use should be true")
	}
	if UseCDATAFromExtensions([]ExtensionNode{{Name: "_xml:cdata", Text: "false"}}) {
		t.Errorf("CDATA should be disabled with false override")
	}
	if !UseCDATAFromExtensions([]ExtensionNode{{Name: "_xml:cdata", Text: "true"}}) {
		t.Errorf("CDATA should be enabled with true override")
	}
	// CDATAUseForItem derives from parent unless overridden
	parent := true
	if !CDATAUseForItem(parent, nil) {
		t.Errorf("item should inherit true by default")
	}
	if CDATAUseForItem(parent, []ExtensionNode{{Name: "_xml:cdata", Text: "false"}}) {
		t.Errorf("item override should disable CDATA")
	}
	// WithCDATAOverride appends marker
	exts := WithCDATAOverride(nil, false)
	found := false
	for _, n := range exts {
		if strings.EqualFold(strings.TrimSpace(n.Name), "_xml:cdata") && strings.TrimSpace(n.Text) == "false" {
			found = true
		}
	}
	if !found {
		t.Errorf("WithCDATAOverride should append _xml:cdata=false")
	}
	// UseCDATAForFeed nil defaults true, feed-level override false
	if !UseCDATAForFeed(nil) {
		t.Errorf("UseCDATAForFeed nil should default to true")
	}
	f := &Feed{Extensions: []ExtensionNode{{Name: "_xml:cdata", Text: "false"}}}
	if UseCDATAForFeed(f) {
		t.Errorf("UseCDATAForFeed should reflect false override")
	}
}

func TestRSSHelpers_ParseAndItemExt(t *testing.T) {
	// rssAuthorString formatting
	if rssAuthorString(nil) != "" {
		t.Errorf("rssAuthorString nil should be empty")
	}
	if rssAuthorString(&Author{Name: "N", Email: "e@x"}) != "e@x (N)" {
		t.Errorf("rssAuthorString name+email formatting unexpected")
	}
	if rssAuthorString(&Author{Name: "", Email: "e@x"}) != "e@x" {
		t.Errorf("rssAuthorString email only unexpected")
	}
	// parsePositiveInt variants
	if v, ok := parsePositiveInt(" 42 "); !ok || v != 42 {
		t.Errorf("parsePositiveInt expected 42 true, got %d %v", v, ok)
	}
	if _, ok := parsePositiveInt("0"); ok {
		t.Errorf("parsePositiveInt zero should be false")
	}
	if _, ok := parsePositiveInt("x"); ok {
		t.Errorf("parsePositiveInt non-number should be false")
	}
	if _, ok := parsePositiveInt("   "); ok {
		t.Errorf("parsePositiveInt empty should be false")
	}
	// itemRSSExtensions mapping
	cat, com, extras := itemRSSExtensions([]ExtensionNode{
		{Name: "_rss:itemCategory", Text: "News"},
		{Name: "_rss:comments", Text: "https://example.org/c"},
		{Name: "x:other", Text: "t"},
	})
	if cat != "News" || com != "https://example.org/c" {
		t.Errorf("itemRSSExtensions expected cat and comments, got %q %q", cat, com)
	}
	if len(extras) != 1 || extras[0].Name != "x:other" {
		t.Errorf("itemRSSExtensions extras pass-through unexpected: %+v", extras)
	}
}

func TestPSPHelpers_NormalizeAndComputePodcastGuid(t *testing.T) {
	// normalizeFeedURL trims schemes and trailing slashes
	if normalizeFeedURL("https://example.com/a/b/") != "example.com/a/b" {
		t.Errorf("normalizeFeedURL should trim scheme and trailing slash")
	}
	if normalizeFeedURL("feed://X/") != "X" {
		t.Errorf("normalizeFeedURL feed:// trim unexpected")
	}
	// computePodcastGuid should be stable on normalized equivalents
	a := computePodcastGuid("https://example.com/podcast.rss")
	b := computePodcastGuid("example.com/podcast.rss/")
	if a != b {
		t.Errorf("computePodcastGuid should be deterministic across normalized inputs, got %q vs %q", a, b)
	}
}

func TestFeedBuilder_SettersTrimAndNilBehavior(t *testing.T) {
	// Build with various setters to exercise trim and nil behaviors
	b := NewFeed(" Title ").
		WithID(" id ").
		WithLink("   ").
		WithFeedURL("  https://example.org/feed  ").
		WithDescription("D").
		WithAuthor("   ", "   ").    // both empty -> nil
		WithImage("  ", "  ", "  "). // all empty -> nil
		WithLanguage("  en-us  ").
		WithCopyright(" C ").
		WithCategories("  A  ", "", "  B ")
	// lenient to avoid profile validations
	f, err := b.WithLenient().Build()
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if f.Title != "Title" {
		t.Errorf("Title trim expected 'Title', got %q", f.Title)
	}
	if f.ID != "id" {
		t.Errorf("ID trim expected 'id', got %q", f.ID)
	}
	if f.Link != nil {
		t.Errorf("WithLink('   ') should set Link=nil")
	}
	if f.FeedURL != "https://example.org/feed" {
		t.Errorf("FeedURL trim expected https://example.org/feed, got %q", f.FeedURL)
	}
	if f.Author != nil {
		t.Errorf("WithAuthor empty should set Author=nil")
	}
	if f.Image != nil {
		t.Errorf("WithImage empty should set Image=nil")
	}
	if f.Language != "en-us" {
		t.Errorf("Language trim expected 'en-us', got %q", f.Language)
	}
	if f.Copyright != " C " {
		t.Errorf("Copyright should be set verbatim, got %q", f.Copyright)
	}
	if len(f.Categories) != 2 || f.Categories[0].Text != "A" || f.Categories[1].Text != "B" {
		t.Errorf("Categories trim/empty filter unexpected: %+v", f.Categories)
	}
}

func TestItemBuilder_SettersTrimAndNilBehavior(t *testing.T) {
	ib := NewItem("  t  ").
		WithID("  id  ").
		WithLink("   ").   // -> nil
		WithSource("   "). // -> nil
		WithDescription("d").
		WithContentHTML("html").
		WithAuthor("   ", "   ").       // -> nil
		WithEnclosure("   ", 0, "   "). // -> nil
		WithDurationSeconds(-5)         // -> 0
	// lenient to avoid strict checks
	it, err := ib.WithLenient().Build()
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if it.Title != "t" {
		t.Errorf("Title trim expected 't', got %q", it.Title)
	}
	if it.ID != "id" {
		t.Errorf("ID trim expected 'id', got %q", it.ID)
	}
	if it.Link != nil {
		t.Errorf("WithLink('   ') should set Link=nil")
	}
	if it.Source != nil {
		t.Errorf("WithSource('   ') should set Source=nil")
	}
	if it.Description != "d" {
		t.Errorf("Description expected 'd', got %q", it.Description)
	}
	if it.Content != "html" {
		t.Errorf("Content expected 'html', got %q", it.Content)
	}
	if it.Author != nil {
		t.Errorf("WithAuthor empty should set Author=nil")
	}
	if it.Enclosure != nil {
		t.Errorf("WithEnclosure empty/type/length invalid should set Enclosure=nil")
	}
	if it.DurationSeconds != 0 {
		t.Errorf("WithDurationSeconds negative should clamp to 0, got %d", it.DurationSeconds)
	}
}

func TestBuilder_AddItemFuncNil_And_WithSortNil_NoOp(t *testing.T) {
	b := NewFeed("t")
	b.AddItemFunc(nil) // no-op
	b.WithSort(nil)    // no-op
	f, err := b.WithLenient().Build()
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if len(f.Items) != 0 {
		t.Errorf("expected no items after AddItemFunc(nil)")
	}
}

// Additional coverage

func TestRunProfileValidations_AggregatesErrors(t *testing.T) {
	// Lenient builder to skip strict title check, but force RSS and JSON validation errors:
	// - RSS: missing description
	// - JSON: missing feed title
	b := NewFeed("").WithLenient().
		WithLink("https://example.org/").
		WithProfiles(ProfileRSS, ProfileJSON)
	// Add one item with an ID so JSON item id isn't the failure cause (feed title is)
	b.AddItem(NewItem("I").WithID("id-1"))
	_, err := b.Build()
	if err == nil {
		t.Fatalf("expected aggregated validation errors for RSS+JSON")
	}
	msg := err.Error()
	if !strings.Contains(msg, "rss: channel title required") || !strings.Contains(msg, "json: feed title required") {
		t.Errorf("expected both RSS and JSON errors aggregated, got: %v", msg)
	}
}

func TestWithSortBy_DefaultFallbackOnUnknownField(t *testing.T) {
	// Unknown field should fallback to created date sorting
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t0.Add(2 * time.Hour)
	b := NewFeed("t")
	b.AddItem(&ItemBuilder{item: Item{Title: "A", Created: t1}, strict: false})
	b.AddItem(&ItemBuilder{item: Item{Title: "B", Created: t2}, strict: false})
	b.AddItem(&ItemBuilder{item: Item{Title: "C", Created: t0}, strict: false})
	b.WithSortBy(ItemSortField(999), SortAsc) // trigger default case
	f, err := b.Build()
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	// Ascending by Created: C(t0), A(t1), B(t2)
	if f.Items[0].Title != "C" || f.Items[1].Title != "A" || f.Items[2].Title != "B" {
		t.Errorf("fallback created sort asc unexpected order: %+v", []string{f.Items[0].Title, f.Items[1].Title, f.Items[2].Title})
	}
}

func TestItemBuilder_StrictTitleOrDescriptionRequired(t *testing.T) {
	// Strict mode: both title and description empty -> error
	ib := NewItem("")
	it, err := ib.Build()
	if err == nil || it != nil {
		t.Fatalf("expected item strict error when title and description are empty")
	}
}

func TestFeedBuilder_StringFormat(t *testing.T) {
	b := NewFeed("Title").AddItem(NewItem("A")).AddItem(NewItem("B"))
	s := b.String()
	if !strings.Contains(s, "Title") || !strings.Contains(s, "Items:2") {
		t.Errorf("builder String() unexpected: %s", s)
	}
}

// Additional low-level helper coverage

func TestItemRSSExtensions_EmptyTextPassThrough(t *testing.T) {
	// When item-level RSS builder markers have empty text, they should be passed through as extras
	cat, com, extras := itemRSSExtensions([]ExtensionNode{
		{Name: "_rss:itemCategory", Text: "   "}, // trimmed empty
		{Name: "_rss:comments", Text: ""},        // empty
		{Name: "x:other", Text: "val"},
	})
	if cat != "" || com != "" {
		t.Errorf("expected empty cat/comments when text trimmed empty, got cat=%q com=%q", cat, com)
	}
	// Expect both empty markers and unknown to remain in extras
	if len(extras) != 3 {
		t.Fatalf("expected 3 extras passthrough, got %d", len(extras))
	}
	names := []string{extras[0].Name, extras[1].Name, extras[2].Name}
	want := map[string]bool{"_rss:itemCategory": true, "_rss:comments": true, "x:other": true}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected extra name: %s", n)
		}
	}
}

func TestParsePositiveInt_NegativeAndSpaces(t *testing.T) {
	if v, ok := parsePositiveInt(" -7 "); ok || v != 0 {
		t.Errorf("parsePositiveInt negative should be false/0, got %d %v", v, ok)
	}
	if v, ok := parsePositiveInt(" 0010 "); !ok || v != 10 {
		t.Errorf("parsePositiveInt leading zeros trimmed expected 10 true, got %d %v", v, ok)
	}
}

func TestConvertCategories_TrimsAndSkips(t *testing.T) {
	cats := []*Category{
		nil,
		{Text: "   "},
		{Text: "  A  "},
		{Text: ""},
		{Text: "B"},
	}
	out := convertCategories(cats)
	if len(out) != 2 {
		t.Fatalf("convertCategories expected 2, got %d", len(out))
	}
	// convertCategories preserves text (no trim), but skips entries that are empty after trimming
	if out[0].Text != "  A  " || out[1].Text != "B" {
		t.Errorf("convertCategories trim/skip unexpected: %#v", out)
	}
}

func TestCDATA_ItemOverride_DisableOnItem(t *testing.T) {
	// Feed default (enabled), item overrides to disable
	f := &Feed{
		Title:       "T",
		Link:        &Link{Href: "https://example.org/"},
		Description: "<p>Channel</p>",
	}
	ib := NewItem("I").WithDescription("<p>Item</p>").WithCreated(time.Now().UTC()).WithXMLCDATA(false)
	it, err := ib.Build()
	if err != nil {
		t.Fatalf("item Build error: %v", err)
	}
	f.Items = []*Item{it}

	rssXML, err := ToRSS(f)
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}
	// Channel still uses CDATA by default
	if !strings.Contains(rssXML, "<description><![CDATA[<p>Channel</p>]]></description>") {
		t.Errorf("channel description should use CDATA by default; got:\n%s", rssXML)
	}
	// Item description should not use CDATA due to item override
	// Extract first item block to avoid matching channel-level description
	start := strings.Index(rssXML, "<item>")
	if start == -1 {
		t.Fatalf("expected <item> element present")
	}
	rest := rssXML[start:]
	end := strings.Index(rest, "</item>")
	if end == -1 {
		t.Fatalf("expected </item> closing tag present")
	}
	itemBlock := rest[:end]
	if strings.Contains(itemBlock, "<![CDATA[") {
		t.Errorf("item description should not use CDATA when item override false; got item block:\n%s", itemBlock)
	}
}

func TestCDATA_ItemOverride_EnableOnItemWhenFeedDisabled(t *testing.T) {
	// Feed-level disabled, item overrides to enable
	f := &Feed{
		Title:       "T",
		Link:        &Link{Href: "https://example.org/"},
		Description: "<p>Channel</p>",
		Extensions:  []ExtensionNode{{Name: "_xml:cdata", Text: "false"}},
	}
	ib := NewItem("I").WithDescription("<p>Item</p>").WithCreated(time.Now().UTC()).WithXMLCDATA(true)
	it, err := ib.Build()
	if err != nil {
		t.Fatalf("item Build error: %v", err)
	}
	f.Items = []*Item{it}

	rssXML, err := ToRSS(f)
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}
	// Channel should not use CDATA
	if strings.Contains(rssXML, "<![CDATA[<p>Channel</p>]]>") {
		t.Errorf("channel description should not use CDATA when feed-level disabled; got:\n%s", rssXML)
	}
	// Item should use CDATA due to item-level override
	if !strings.Contains(rssXML, "<content:encoded><![CDATA[") && !strings.Contains(rssXML, "<description><![CDATA[<p>Item</p>]]></description>") {
		// At least description must be CDATA
		t.Errorf("item description should use CDATA when item override true; got:\n%s", rssXML)
	}
}
