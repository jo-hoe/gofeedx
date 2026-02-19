package gofeedx

import (
	"strings"
	"testing"
	"time"
)

func TestRSSBuilder_Helpers_ChannelAndItemFields(t *testing.T) {
	// Build feed using RSS-specific builder helpers
	b := NewFeed("RSS Title").
		WithLink("https://example.org/").
		WithDescription("Desc").
		WithLanguage("en-us").
		WithImage("https://example.org/logo.png", "", "").
		WithRSSTTL(60).
		WithRSSImageSize(144, 144).
		WithRSSCategory("OverrideCat").
		WithRSSWebMaster("webmaster@example.org").
		WithRSSGenerator("gofeedx").
		WithRSSDocs("https://example.org/docs").
		WithRSSCloud("cloud svc").
		WithRSSRating("PG").
		WithRSSSkipHours("1 2").
		WithRSSSkipDays("Mon Tue")

	ib := NewItem("Item 1").
		WithDescription("Item Desc").
		WithCreated(time.Now().UTC()).
		WithRSSItemCategory("ItemCat").
		WithRSSComments("https://example.org/comments/1")
	b.AddItem(ib)

	f, err := b.WithProfiles(ProfileRSS).Build()
	if err != nil {
		t.Fatalf("Build() unexpected error: %v", err)
	}
	xml, err := ToRSS(f)
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}

	// Channel-level checks
	if !strings.Contains(xml, "<ttl>60</ttl>") {
		t.Errorf("expected <ttl>60</ttl> in channel")
	}
	if !strings.Contains(xml, "<category>OverrideCat</category>") {
		t.Errorf("expected channel category override")
	}
	if !strings.Contains(xml, "<webMaster>webmaster@example.org</webMaster>") {
		t.Errorf("expected webMaster element")
	}
	if !strings.Contains(xml, "<generator>gofeedx</generator>") {
		t.Errorf("expected generator element")
	}
	if !strings.Contains(xml, "<docs>https://example.org/docs</docs>") {
		t.Errorf("expected docs element")
	}
	if !strings.Contains(xml, "<cloud>cloud svc</cloud>") {
		t.Errorf("expected cloud element")
	}
	if !strings.Contains(xml, "<rating>PG</rating>") {
		t.Errorf("expected rating element")
	}
	if !strings.Contains(xml, "<skipHours>1 2</skipHours>") {
		t.Errorf("expected skipHours element")
	}
	if !strings.Contains(xml, "<skipDays>Mon Tue</skipDays>") {
		t.Errorf("expected skipDays element")
	}

	// Image size mapping
	if !strings.Contains(xml, "<image>") || !strings.Contains(xml, "<width>144</width>") || !strings.Contains(xml, "<height>144</height>") {
		t.Errorf("expected image with width/height from WithRSSImageSize")
	}

	// Item-level checks for helpers
	if !strings.Contains(xml, "<item>") {
		t.Fatalf("expected an item in RSS output")
	}
	if !strings.Contains(xml, "<category>ItemCat</category>") {
		t.Errorf("expected item category from WithRSSItemCategory")
	}
	if !strings.Contains(xml, "<comments>https://example.org/comments/1</comments>") {
		t.Errorf("expected comments element from WithRSSComments")
	}
}