package gofeedx

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJSONAttachment_MarshalJSON_DurationOptional(t *testing.T) {
	att := JSONAttachment{
		Url:      "https://cdn.example.org/a.mp3",
		MIMEType: "audio/mpeg",
		Size:     123,
		Duration: 0,
	}
	b1, err := json.Marshal(&att)
	if err != nil {
		t.Fatalf("json.Marshal(att) error: %v", err)
	}
	if strings.Contains(string(b1), "duration_in_seconds") {
		t.Errorf("duration_in_seconds should be omitted when Duration == 0")
	}
	att.Duration = 5 * time.Second
	b2, err := json.Marshal(&att)
	if err != nil {
		t.Fatalf("json.Marshal(att) with duration error: %v", err)
	}
	if !strings.Contains(string(b2), `"duration_in_seconds":5`) {
		t.Errorf("expected duration_in_seconds==5, got %s", string(b2))
	}
}

func TestJSONItem_AttachmentsMapping_SizeCappedAndDuration(t *testing.T) {
	// Build a feed with one item that has a non-image enclosure -> becomes attachment.
	// Length exceeds maxSize -> capped. DurationSeconds -> duration_in_seconds.
	enclosureLen := int64(maxSize) + 100 // force capping
	item := &Item{
		Title:           "Episode",
		Link:            &Link{Href: "https://example.org/ep"},
		ID:              "id-1",
		Created:         time.Now().UTC(),
		Enclosure:       &Enclosure{Url: "https://cdn.example.org/ep.mp3", Type: "audio/mpeg", Length: enclosureLen},
		DurationSeconds: 7,
	}
	feed := &Feed{
		Title: "JSON feed",
		Items: []*Item{item},
	}

	js, err := (&JSON{Feed: feed}).ToJSONString()
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
	// size should be capped to maxSize
	if a0["size"] != float64(maxSize) {
		t.Errorf("attachment size expected %d, got %v", maxSize, a0["size"])
	}
	if a0["duration_in_seconds"] != float64(7) {
		t.Errorf("attachment duration_in_seconds expected 7, got %v", a0["duration_in_seconds"])
	}
}