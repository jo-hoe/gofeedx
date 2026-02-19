package gofeedx_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jo-hoe/gofeedx"
)

type jfAuthor struct {
	Name   string `json:"name,omitempty"`
	Url    string `json:"url,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

func TestJSONBuilder_Helpers_TopLevel(t *testing.T) {
	// Build feed using JSON-specific builder helpers
	b := gofeedx.NewFeed("JSON Title").
		WithLink("https://example.org/").
		WithDescription("Desc").
		WithLanguage("en").
		WithJSONUserComment("Hello Users").
		WithJSONNextURL("https://example.org/next").
		WithJSONExpired(true).
		WithJSONHub("rssCloud", "https://example.org/hub").
		WithJSONIcon("https://example.org/icon.png").
		WithJSONFavicon("https://example.org/favicon.ico")

	// Minimal item with explicit ID to satisfy validation when building with ProfileJSON
	ib := gofeedx.NewItem("Item 1").
		WithID("id-1").
		WithCreated(time.Now().UTC())
	b.AddItem(ib)

	f, err := b.WithProfiles(gofeedx.ProfileJSON).Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	js, err := gofeedx.ToJSON(f)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(js), &obj); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if obj["user_comment"] != "Hello Users" {
		t.Errorf("expected user_comment from WithJSONUserComment, got %v", obj["user_comment"])
	}
	if obj["next_url"] != "https://example.org/next" {
		t.Errorf("expected next_url from WithJSONNextURL, got %v", obj["next_url"])
	}
	if obj["icon"] != "https://example.org/icon.png" {
		t.Errorf("expected icon override from WithJSONIcon, got %v", obj["icon"])
	}
	if obj["favicon"] != "https://example.org/favicon.ico" {
		t.Errorf("expected favicon override from WithJSONFavicon, got %v", obj["favicon"])
	}
	// expired should be true pointer
	if exp, ok := obj["expired"].(bool); !ok || !exp {
		t.Errorf("expected expired=true, got %v", obj["expired"])
	}
	// hubs should be an array with one entry
	if hubs, ok := obj["hubs"].([]any); !ok || len(hubs) != 1 {
		t.Fatalf("expected one hub entry, got %v", obj["hubs"])
	}
}

func TestJSONBuilder_Helpers_ItemLevel(t *testing.T) {
	b := gofeedx.NewFeed("JSON Title").
		WithLink("https://example.org/")

	ib := gofeedx.NewItem("Item 1").
		WithID("id-1").
		WithCreated(time.Now().UTC()).
		WithJSONContentText("Plain text").
		WithJSONBannerImage("https://example.org/banner.jpg").
		WithJSONTags("tag1", "tag2").
		WithJSONTag("tag3").
		WithJSONImage("https://example.org/img.png")
	b.AddItem(ib)

	f, err := b.WithProfiles(gofeedx.ProfileJSON).Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	js, err := gofeedx.ToJSON(f)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// quick checks
	if !strings.Contains(js, `"content_text": "Plain text"`) {
		t.Errorf("expected content_text from WithJSONContentText")
	}
	if !strings.Contains(js, `"banner_image": "https://example.org/banner.jpg"`) {
		t.Errorf("expected banner_image from WithJSONBannerImage")
	}
	// tags array should contain 3 entries
	if strings.Count(js, `"tags":`) == 0 ||
		!strings.Contains(js, `"tag1"`) ||
		!strings.Contains(js, `"tag2"`) ||
		!strings.Contains(js, `"tag3"`) {
		t.Errorf("expected tags array to include tag1, tag2, tag3")
	}
	if !strings.Contains(js, `"image": "https://example.org/img.png"`) {
		t.Errorf("expected image override from WithJSONImage")
	}
}

func TestJSONAttachment_MarshalJSON_DurationOptional(t *testing.T) {
	// Build a feed with one item and non-image enclosure to produce attachments.
	item := &gofeedx.Item{
		Title:   "Episode",
		Link:    &gofeedx.Link{Href: "https://example.org/ep"},
		ID:      "id-1",
		Created: time.Now().UTC(),
		Enclosure: &gofeedx.Enclosure{
			Url:    "https://cdn.example.org/a.mp3",
			Type:   "audio/mpeg",
			Length: 123,
		},
		DurationSeconds: 0,
	}
	feed := &gofeedx.Feed{
		Title: "JSON feed",
		Items: []*gofeedx.Item{item},
	}

	// Case 1: DurationSeconds == 0 -> duration_in_seconds omitted
	js1, err := gofeedx.ToJSON(feed)
	if err != nil {
		t.Fatalf("ToJSONString() error: %v", err)
	}
	if strings.Contains(js1, "duration_in_seconds") {
		t.Errorf("duration_in_seconds should be omitted when DurationSeconds == 0")
	}

	// Case 2: DurationSeconds > 0 -> duration_in_seconds present
	item.DurationSeconds = 5
	js2, err := gofeedx.ToJSON(feed)
	if err != nil {
		t.Fatalf("ToJSONString() error (with duration): %v", err)
	}
	// Parse JSON to robustly assert the numeric value (indentation/spacing may vary)
	var obj2 map[string]any
	if err := json.Unmarshal([]byte(js2), &obj2); err != nil {
		t.Fatalf("json.Unmarshal (case2): %v", err)
	}
	items2, _ := obj2["items"].([]any)
	if len(items2) == 0 {
		t.Fatalf("expected items array (case2)")
	}
	first2, _ := items2[0].(map[string]any)
	atts2, _ := first2["attachments"].([]any)
	if len(atts2) != 1 {
		t.Fatalf("expected one attachment (case2), got %d", len(atts2))
	}
	a02, _ := atts2[0].(map[string]any)
	if a02["duration_in_seconds"] != float64(5) {
		t.Errorf("expected duration_in_seconds==5, got %v", a02["duration_in_seconds"])
	}
}

func TestJSONItem_AttachmentsMapping_SizeCappedAndDuration(t *testing.T) {
	// Build a feed with one item that has a non-image enclosure -> becomes attachment.
	// Length exceeds maxSize -> capped. DurationSeconds -> duration_in_seconds.
	enclosureLen := int64(2147483647) + 100 // using cap logic implicitly via json.go
	item := &gofeedx.Item{
		Title:           "Episode",
		Link:            &gofeedx.Link{Href: "https://example.org/ep"},
		ID:              "id-1",
		Created:         time.Now().UTC(),
		Enclosure:       &gofeedx.Enclosure{Url: "https://cdn.example.org/ep.mp3", Type: "audio/mpeg", Length: enclosureLen},
		DurationSeconds: 7,
	}
	feed := &gofeedx.Feed{
		Title: "JSON feed",
		Items: []*gofeedx.Item{item},
	}

	js, err := (&gofeedx.JSON{Feed: feed}).ToJSONString()
	if err != nil {
		t.Fatalf("ToJSONString() error: %v", err)
	}

	// Unmarshal loosely to inspect attachments
	var obj map[string]any
	if err := json.Unmarshal([]byte(js), &obj); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	items, _ := obj["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("expected items array")
	}
	first, _ := items[0].(map[string]any)
	atts, _ := first["attachments"].([]any)
	if len(atts) != 1 {
		t.Fatalf("expected one attachment, got %d", len(atts))
	}
	a0, _ := atts[0].(map[string]any)
	if a0["url"] != "https://cdn.example.org/ep.mp3" {
		t.Errorf("attachment url got %v", a0["url"])
	}
	if a0["mime_type"] != "audio/mpeg" {
		t.Errorf("attachment mime_type got %v", a0["mime_type"])
	}
	// size should be capped to maxSize (int32 cap reflected as float64 in JSON)
	if a0["size"] != float64(2147483647) {
		t.Errorf("attachment size expected %d, got %v", 2147483647, a0["size"])
	}
	if a0["duration_in_seconds"] != float64(7) {
		t.Errorf("attachment duration_in_seconds expected 7, got %v", a0["duration_in_seconds"])
	}
}

type jfItem struct {
	Id            string      `json:"id"`
	Url           string      `json:"url,omitempty"`
	ExternalUrl   string      `json:"external_url,omitempty"`
	Title         string      `json:"title,omitempty"`
	ContentHTML   string      `json:"content_html,omitempty"`
	ContentText   string      `json:"content_text,omitempty"`
	Summary       string      `json:"summary,omitempty"`
	Image         string      `json:"image,omitempty"`
	BannerImage   string      `json:"banner_image,omitempty"`
	PublishedDate string      `json:"date_published,omitempty"`
	ModifiedDate  string      `json:"date_modified,omitempty"`
	Authors       []jfAuthor  `json:"authors,omitempty"`
	Tags          []string    `json:"tags,omitempty"`
	Attachments   interface{} `json:"attachments,omitempty"`
	// flattened extensions may also appear here with arbitrary keys
}

type jfFeed struct {
	Version     string      `json:"version"`
	Title       string      `json:"title"`
	Language    string      `json:"language,omitempty"`
	HomePageUrl string      `json:"home_page_url,omitempty"`
	FeedUrl     string      `json:"feed_url,omitempty"`
	Description string      `json:"description,omitempty"`
	UserComment string      `json:"user_comment,omitempty"`
	NextUrl     string      `json:"next_url,omitempty"`
	Icon        string      `json:"icon,omitempty"`
	Favicon     string      `json:"favicon,omitempty"`
	Authors     []jfAuthor  `json:"authors,omitempty"`
	Expired     *bool       `json:"expired,omitempty"`
	Hubs        interface{} `json:"hubs,omitempty"`
	Items       []jfItem    `json:"items,omitempty"`
	// flattened extensions may also appear here with arbitrary keys
}

func newJSONBaseFeed() *gofeedx.Feed {
	return &gofeedx.Feed{
		Title:       "Example JSON Feed",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "JSON Feed 1.1 test feed.",
		Language:    "en",
		Created:     time.Now().UTC(),
		Updated:     time.Now().UTC(),
		Author:      &gofeedx.Author{Name: "Feed Author"},
	}
}

func newJSONBaseItem() *gofeedx.Item {
	return &gofeedx.Item{
		Title:       "First Post",
		Description: "Summary here",
		Content:     "<p>Content HTML</p>",
		Link:        &gofeedx.Link{Href: "https://example.org/posts/1"},
		Source:      &gofeedx.Link{Href: "https://mirror.example.org/posts/1"},
		Author:      &gofeedx.Author{Name: "Entry Author"},
		ID:          "post-1",
		Created:     time.Now().UTC().Add(-1 * time.Hour),
		Updated:     time.Now().UTC(),
	}
}

func TestJSONFeedRequiredFields(t *testing.T) {
	f := newJSONBaseFeed()
	f.Items = append(f.Items, newJSONBaseItem())

	js, err := gofeedx.ToJSON(f)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(js), &doc); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	// Required top-level fields
	if v, ok := doc["version"].(string); !ok || v != "https://jsonfeed.org/version/1.1" {
		t.Errorf("version must be https://jsonfeed.org/version/1.1, got %#v", doc["version"])
	}
	if v, ok := doc["title"].(string); !ok || strings.TrimSpace(v) == "" {
		t.Errorf("title is required and must be non-empty")
	}
}

func TestJSONFeedItemsAndIds(t *testing.T) {
	f := newJSONBaseFeed()
	item := newJSONBaseItem()
	f.Items = append(f.Items, item)

	js, err := gofeedx.ToJSON(f)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var feed jfFeed
	if err := json.Unmarshal([]byte(js), &feed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(feed.Items) == 0 {
		t.Fatalf("expected at least one item")
	}
	if strings.TrimSpace(feed.Items[0].Id) == "" {
		t.Errorf("JSON Feed spec: each item must have a non-empty id")
	}
}

func TestJSONFeedDatesRFC3339(t *testing.T) {
	f := newJSONBaseFeed()
	item := newJSONBaseItem()
	f.Items = append(f.Items, item)

	js, err := gofeedx.ToJSON(f)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var feed jfFeed
	if err := json.Unmarshal([]byte(js), &feed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := feed.Items[0]
	if got.PublishedDate == "" {
		t.Errorf("expected date_published for created time")
	} else if _, err := time.Parse(time.RFC3339, got.PublishedDate); err != nil {
		t.Errorf("date_published must be RFC3339, got %q: %v", got.PublishedDate, err)
	}
	if got.ModifiedDate == "" {
		t.Errorf("expected date_modified for updated time")
	} else if _, err := time.Parse(time.RFC3339, got.ModifiedDate); err != nil {
		t.Errorf("date_modified must be RFC3339, got %q: %v", got.ModifiedDate, err)
	}
}

func TestJSONFeedAuthorsV11(t *testing.T) {
	f := newJSONBaseFeed()
	item := newJSONBaseItem()
	f.Items = append(f.Items, item)

	js, err := gofeedx.ToJSON(f)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	var feed jfFeed
	if err := json.Unmarshal([]byte(js), &feed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Feed-level authors array v1.1
	if len(feed.Authors) == 0 || feed.Authors[0].Name != "Feed Author" {
		t.Errorf("expected feed authors[0].name == Feed Author, got %#v", feed.Authors)
	}
	// Item-level authors array v1.1
	if len(feed.Items) == 0 || len(feed.Items[0].Authors) == 0 || feed.Items[0].Authors[0].Name != "Entry Author" {
		t.Errorf("expected item authors[0].name == Entry Author, got %#v", feed.Items[0].Authors)
	}
}

func TestJSONFeedContentHTMLAndImageMapping(t *testing.T) {
	f := newJSONBaseFeed()
	item := newJSONBaseItem()
	// Add image enclosure to map to "image"
	item.Enclosure = &gofeedx.Enclosure{
		Url:    "https://cdn.example.org/img.png",
		Type:   "image/png",
		Length: 100,
	}
	f.Items = append(f.Items, item)

	js, err := gofeedx.ToJSON(f)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	var feed jfFeed
	if err := json.Unmarshal([]byte(js), &feed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := feed.Items[0]
	if got.ContentHTML == "" || !strings.Contains(got.ContentHTML, "Content HTML") {
		t.Errorf("expected content_html to carry Item.Content HTML")
	}
	if got.Image != "https://cdn.example.org/img.png" {
		t.Errorf("expected image mapped from image enclosure, got %q", got.Image)
	}
}

func TestJSONFeedExtensionsFlattened(t *testing.T) {
	f := newJSONBaseFeed()
	f.Extensions = []gofeedx.ExtensionNode{
		{Name: "x-top", Text: "top"},
	}
	item := newJSONBaseItem()
	item.Extensions = []gofeedx.ExtensionNode{
		{Name: "x-item", Text: "ival"},
	}
	f.Items = append(f.Items, item)

	js, err := gofeedx.ToJSON(f)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Decode to generic map to check flattened keys
	var doc map[string]any
	if err := json.Unmarshal([]byte(js), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, ok := doc["x-top"].(string); !ok || v != "top" {
		t.Errorf("expected flattened extension x-top at top-level, got %#v", doc["x-top"])
	}
	arr, ok := doc["items"].([]any)
	if !ok || len(arr) == 0 {
		t.Fatalf("expected items array")
	}
	first, ok := arr[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first item to be object")
	}
	if v, ok := first["x-item"].(string); !ok || v != "ival" {
		t.Errorf("expected flattened extension x-item at item-level, got %#v", first["x-item"])
	}
}

// Encode the rule that item.id must be non-empty; the library currently allows empty id.
// This test will fail until enforcement is added, which is acceptable per task instructions.
func TestJSONFeedItemIdMustBeNonEmptyPerSpec(t *testing.T) {
	f := newJSONBaseFeed()
	item := newJSONBaseItem()
	item.ID = "" // non-conformant per spec
	f.Items = append(f.Items, item)

	js, err := gofeedx.ToJSON(f)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	var feed jfFeed
	if err := json.Unmarshal([]byte(js), &feed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if feed.Items[0].Id == "" {
		t.Errorf("JSON Feed spec: item.id must be a non-empty string; got empty")
	}
}

func TestJSONFeedDoesNotIncludePSPFields(t *testing.T) {
	f := newJSONBaseFeed()
	item := newJSONBaseItem()
	f.Items = append(f.Items, item)

	// Configure some generic fields; ensure PSP-only fields do not leak into JSON
	f.FeedURL = "https://example.com/podcast.rss"
	f.Categories = append(f.Categories, &gofeedx.Category{Text: "Technology"})
	item.DurationSeconds = 99

	js, err := gofeedx.ToJSON(f)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	// Quick string checks
	if strings.Contains(js, "itunes:") || strings.Contains(js, "podcast:") {
		t.Errorf("unexpected itunes:/podcast: keys present in JSON output")
	}

	// Decode and assert specific PSP keys aren't present
	var obj map[string]any
	if err := json.Unmarshal([]byte(js), &obj); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	disallowedTop := []string{"itunes:explicit", "itunes:image", "itunes:type", "podcast:locked", "podcast:guid", "podcast:funding", "podcast:txt"}
	for _, k := range disallowedTop {
		if _, ok := obj[k]; ok {
			t.Errorf("unexpected PSP key at top-level JSON: %s", k)
		}
	}
	items, _ := obj["items"].([]any)
	if len(items) > 0 {
		first, _ := items[0].(map[string]any)
		disallowedItem := []string{"itunes:duration", "itunes:episode", "itunes:season", "itunes:episodeType", "itunes:block", "podcast:transcript"}
		for _, k := range disallowedItem {
			if _, ok := first[k]; ok {
				t.Errorf("unexpected PSP key at item-level JSON: %s", k)
			}
		}
	}
}

func TestValidateJSON_Success(t *testing.T) {
	f := &gofeedx.Feed{
		Title: "JSON Title",
	}
	f.Items = append(f.Items, &gofeedx.Item{
		ID:    "item-1",
		Title: "First",
	})
	if err := gofeedx.ValidateJSON(f); err != nil {
		t.Fatalf("ValidateJSON() unexpected error: %v", err)
	}
}

func TestValidateJSON_MissingTitle(t *testing.T) {
	f := &gofeedx.Feed{}
	f.Items = append(f.Items, &gofeedx.Item{ID: "x"})
	err := gofeedx.ValidateJSON(f)
	if err == nil || !strings.Contains(err.Error(), "feed title required") {
		t.Fatalf("ValidateJSON() expected missing title error, got: %v", err)
	}
}

func TestValidateJSON_NoItems(t *testing.T) {
	f := &gofeedx.Feed{
		Title: "JSON Title",
	}
	err := gofeedx.ValidateJSON(f)
	if err == nil || !strings.Contains(err.Error(), "at least one item required") {
		t.Fatalf("ValidateJSON() expected at least one item error, got: %v", err)
	}
}

func TestValidateJSON_ItemIdRequired(t *testing.T) {
	f := &gofeedx.Feed{
		Title: "JSON Title",
	}
	// Invalid: empty item ID per spec
	f.Items = append(f.Items, &gofeedx.Item{Title: "x"})
	err := gofeedx.ValidateJSON(f)
	if err == nil || !strings.Contains(err.Error(), "item[0] id required") {
		t.Fatalf("ValidateJSON() expected item id required error, got: %v", err)
	}
}
