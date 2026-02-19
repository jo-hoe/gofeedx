package gofeedx

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJSONBuilder_Helpers_TopLevel(t *testing.T) {
	// Build feed using JSON-specific builder helpers
	b := NewFeed("JSON Title").
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
	ib := NewItem("Item 1").
		WithID("id-1").
		WithCreated(time.Now().UTC())
	b.AddItem(ib)

	f, err := b.WithProfiles(ProfileJSON).Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	js, err := ToJSON(f)
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
	b := NewFeed("JSON Title").
		WithLink("https://example.org/")

	ib := NewItem("Item 1").
		WithID("id-1").
		WithCreated(time.Now().UTC()).
		WithJSONContentText("Plain text").
		WithJSONBannerImage("https://example.org/banner.jpg").
		WithJSONTags("tag1", "tag2").
		WithJSONTag("tag3").
		WithJSONImage("https://example.org/img.png")
	b.AddItem(ib)

	f, err := b.WithProfiles(ProfileJSON).Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	js, err := ToJSON(f)
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
	if strings.Count(js, `"tags":`) == 0 || !(strings.Contains(js, `"tag1"`) && strings.Contains(js, `"tag2"`) && strings.Contains(js, `"tag3"`)) {
		t.Errorf("expected tags array to include tag1, tag2, tag3")
	}
	if !strings.Contains(js, `"image": "https://example.org/img.png"`) {
		t.Errorf("expected image override from WithJSONImage")
	}
}