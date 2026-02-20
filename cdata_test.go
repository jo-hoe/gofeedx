package gofeedx_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jo-hoe/gofeedx"
)

func buildFeedForCDATA() *gofeedx.Feed {
	now := time.Now().UTC()
	f := &gofeedx.Feed{
		Title:       "T",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "<p>Channel</p>",
		Language:    "en-us",
		Created:     now,
	}
	item := &gofeedx.Item{
		Title:       "I",
		Description: "<p>Item</p>",
		Content:     "<p>Body</p>",
		Created:     now,
	}
	// Enclosure optional for RSS/Atom; add for PSP compatibility if needed in future
	item.Enclosure = &gofeedx.Enclosure{
		Url:    "https://cdn.example.org/x.mp3",
		Type:   "audio/mpeg",
		Length: 1234,
	}
	f.Items = []*gofeedx.Item{item}
	return f
}

func TestCDATA_DefaultEnabled_RSS_Atom(t *testing.T) {

	f := buildFeedForCDATA()

	// RSS asserts
	rssXML, err := gofeedx.ToRSS(f)
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}
	if !strings.Contains(rssXML, "<description><![CDATA[<p>Channel</p>]]></description>") {
		t.Errorf("RSS channel description should use CDATA when it contains HTML; got:\n%s", rssXML)
	}
	if !strings.Contains(rssXML, "<item>") || !strings.Contains(rssXML, "<description><![CDATA[<p>Item</p>]]></description>") {
		t.Errorf("RSS item description should use CDATA when it contains HTML; got:\n%s", rssXML)
	}
	if !strings.Contains(rssXML, "<content:encoded><![CDATA[<p>Body</p>]]></content:encoded>") {
		t.Errorf("RSS content:encoded should use CDATA by default; got:\n%s", rssXML)
	}

	// Atom asserts
	atomXML, err := gofeedx.ToAtom(f)
	if err != nil {
		t.Fatalf("ToAtom failed: %v", err)
	}
	// Feed subtitle and entry summary/content should use CDATA when enabled
	if !strings.Contains(atomXML, "<subtitle><![CDATA[<p>Channel</p>]]></subtitle>") {
		t.Errorf("Atom feed subtitle should use CDATA when it contains HTML; got:\n%s", atomXML)
	}
	if !strings.Contains(atomXML, `<summary type="html"><![CDATA[<p>Item</p>]]></summary>`) {
		t.Errorf("Atom entry summary should use CDATA when it contains HTML; got:\n%s", atomXML)
	}
	if !strings.Contains(atomXML, `<content type="html"><![CDATA[<p>Body</p>]]></content>`) {
		t.Errorf("Atom entry content should use CDATA when it contains HTML; got:\n%s", atomXML)
	}
}

func TestCDATA_Disabled_Escapes_RSS_Atom(t *testing.T) {
	// Disable CDATA via builder/extension
	f := buildFeedForCDATA()
	f.Extensions = append(f.Extensions, gofeedx.ExtensionNode{Name: "_xml:cdata", Text: "false"})

	// RSS asserts
	rssXML, err := gofeedx.ToRSS(f)
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}
	// No CDATA anywhere when disabled
	if strings.Contains(rssXML, "<![CDATA[") {
		t.Errorf("RSS should not emit CDATA when disabled; got:\n%s", rssXML)
	}
	// Presence checks for elements (content must be escaped but we avoid brittle substring matching)
	if !strings.Contains(rssXML, "<description>") {
		t.Errorf("RSS channel description element missing; got:\n%s", rssXML)
	}
	if !strings.Contains(rssXML, "<item>") {
		t.Errorf("RSS item element missing; got:\n%s", rssXML)
	}
	if !strings.Contains(rssXML, "<content:encoded>") {
		t.Errorf("RSS content:encoded element missing; got:\n%s", rssXML)
	}

	// Atom asserts
	atomXML, err := gofeedx.ToAtom(f)
	if err != nil {
		t.Fatalf("ToAtom failed: %v", err)
	}
	if strings.Contains(atomXML, "<![CDATA[") {
		t.Errorf("Atom should not emit CDATA when disabled; got:\n%s", atomXML)
	}
	// Presence checks
	if !strings.Contains(atomXML, "<subtitle>") {
		t.Errorf("Atom feed subtitle element missing; got:\n%s", atomXML)
	}
	if !strings.Contains(atomXML, `<summary type="html">`) {
		t.Errorf("Atom entry summary element missing; got:\n%s", atomXML)
	}
	if !strings.Contains(atomXML, `<content type="html">`) {
		t.Errorf("Atom entry content element missing; got:\n%s", atomXML)
	}
}

func TestCDATA_AlreadyWrapped_NotDoubleWrapped(t *testing.T) {

	now := time.Now().UTC()
	f := &gofeedx.Feed{
		Title:       "T",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "<![CDATA[<em>C</em>]]>", // already wrapped
		Created:     now,
	}
	item := &gofeedx.Item{
		Title:       "I",
		Description: "<![CDATA[<em>I</em>]]>",
		Content:     "<![CDATA[<em>B</em>]]>",
		Created:     now,
	}
	f.Items = []*gofeedx.Item{item}

	// RSS: Expect single CDATA wrapper, not nested
	rssXML, err := gofeedx.ToRSS(f)
	if err != nil {
		t.Fatalf("ToRSS failed: %v", err)
	}
	if !strings.Contains(rssXML, "<description><![CDATA[<em>C</em>]]></description>") {
		t.Errorf("RSS channel description should remain single CDATA-wrapped; got:\n%s", rssXML)
	}
	if !strings.Contains(rssXML, "<description><![CDATA[<em>I</em>]]></description>") {
		t.Errorf("RSS item description should remain single CDATA-wrapped; got:\n%s", rssXML)
	}
	if !strings.Contains(rssXML, "<content:encoded><![CDATA[<em>B</em>]]></content:encoded>") {
		t.Errorf("RSS content:encoded should remain single CDATA-wrapped; got:\n%s", rssXML)
	}

	// Atom: summary/content should remain single-wrapped
	atomXML, err := gofeedx.ToAtom(f)
	if err != nil {
		t.Fatalf("ToAtom failed: %v", err)
	}
	if !strings.Contains(atomXML, "<subtitle><![CDATA[<em>C</em>]]></subtitle>") {
		t.Errorf("Atom feed subtitle should remain single CDATA-wrapped; got:\n%s", atomXML)
	}
	if !strings.Contains(atomXML, `<summary type="html"><![CDATA[<em>I</em>]]></summary>`) {
		t.Errorf("Atom entry summary should remain single CDATA-wrapped; got:\n%s", atomXML)
	}
	if !strings.Contains(atomXML, `<content type="html"><![CDATA[<em>B</em>]]></content>`) {
		t.Errorf("Atom entry content should remain single CDATA-wrapped; got:\n%s", atomXML)
	}
}