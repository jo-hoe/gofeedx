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
	f.Add(newJSONBaseItem())

	js, err := f.ToJSON()
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
	f.Add(item)

	js, err := f.ToJSON()
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
	f.Add(item)

	js, err := f.ToJSON()
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
	f.Add(item)

	js, err := f.ToJSON()
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
	f.Add(item)

	js, err := f.ToJSON()
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
	f.Add(item)

	js, err := f.ToJSON()
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
	f.Add(item)

	js, err := f.ToJSON()
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
	f.Add(item)

	// Configure PSP-only fields that should not leak into JSON Feed
	explicit := true
	f.FeedURL = "https://example.com/podcast.rss"
	f.ItunesImageHref = "https://example.com/artwork.jpg"
	f.ItunesExplicit = &explicit
	f.ItunesType = "episodic"
	f.Categories = append(f.Categories, &gofeedx.Category{Text: "Technology"})
	f.PodcastLocked = &explicit
	f.PodcastGuid = "a-guid"
	f.PodcastFunding = &gofeedx.PodcastFunding{Url: "https://example.com/fund", Text: "Fund us"}
	f.PodcastTXT = &gofeedx.PodcastTXT{Purpose: "verify", Value: "token"}

	item.DurationSeconds = 99
	item.ItunesEpisodeType = "bonus"

	js, err := f.ToJSON()
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
