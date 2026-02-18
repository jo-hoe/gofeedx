package gofeedx_test

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/jo-hoe/gofeedx"
)

type atomLink struct {
	Href   string `xml:"href,attr"`
	Rel    string `xml:"rel,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr"`
}

type atomEntry struct {
	Title   string      `xml:"title"`
	Id      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Links   []atomLink  `xml:"link"`
	Summary *atomInline `xml:"summary"`
	Content *atomInline `xml:"content"`
	Author  *atomPerson `xml:"author"`
}

type atomPerson struct {
	Name  string `xml:"name"`
	Email string `xml:"email"`
}

type atomInline struct {
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}

type atomFeedDoc struct {
	XMLName xml.Name   `xml:"feed"`
	Xmlns   string     `xml:"xmlns,attr"`
	Title   string     `xml:"title"`
	Id      string     `xml:"id"`
	Updated string     `xml:"updated"`
	Author  *atomPerson `xml:"author"`
	Entries []atomEntry `xml:"entry"`
}

func newAtomBaseFeed() *gofeedx.Feed {
	return &gofeedx.Feed{
		Title:       "Example Atom Feed",
		Link:        &gofeedx.Link{Href: "https://example.org/"},
		Description: "An example feed for testing.",
		Created:     time.Now().UTC(),
	}
}

func newAtomBaseItem() *gofeedx.Item {
	return &gofeedx.Item{
		Title:   "Entry 1",
		Link:    &gofeedx.Link{Href: "https://example.org/entry1"},
		Created: time.Now().UTC(),
	}
}

func TestAtomFeedRequiredElements(t *testing.T) {
	f := newAtomBaseFeed()
	f.Add(newAtomBaseItem())

	xmlStr, err := f.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom failed: %v", err)
	}

	// Basic string checks for namespace and structure
	if !strings.Contains(xmlStr, `xmlns="http://www.w3.org/2005/Atom"`) {
		t.Errorf("missing Atom namespace on feed root")
	}
	if !strings.Contains(xmlStr, "<title>Example Atom Feed</title>") {
		t.Errorf("missing feed title")
	}

	var doc atomFeedDoc
	if err := xml.Unmarshal([]byte(xmlStr), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if doc.Xmlns != "http://www.w3.org/2005/Atom" {
		t.Errorf("feed xmlns = %q, want %q", doc.Xmlns, "http://www.w3.org/2005/Atom")
	}
	if strings.TrimSpace(doc.Id) == "" {
		t.Errorf("feed id must be present (RFC 4287 4.1.1)")
	}
	if _, err := time.Parse(time.RFC3339, doc.Updated); err != nil {
		t.Errorf("feed updated must be RFC3339, got %q: %v", doc.Updated, err)
	}
	if len(doc.Entries) == 0 {
		t.Fatalf("expected at least one entry")
	}
	e := doc.Entries[0]
	if strings.TrimSpace(e.Id) == "" {
		t.Errorf("entry id must be present (RFC 4287 4.1.2)")
	}
	if e.Title == "" {
		t.Errorf("entry title must be present (RFC 4287 4.1.2)")
	}
	if _, err := time.Parse(time.RFC3339, e.Updated); err != nil {
		t.Errorf("entry updated must be RFC3339, got %q: %v", e.Updated, err)
	}
	// When item has Link and no explicit rel, library defaults to rel=alternate
	foundAlt := false
	for _, l := range e.Links {
		if l.Rel == "alternate" && l.Href == "https://example.org/entry1" {
			foundAlt = true
			break
		}
	}
	if !foundAlt {
		t.Errorf("expected rel=alternate link to entry URL to be present")
	}
}

func TestAtomEntryContentAndSummaryBehavior(t *testing.T) {
	// Only Description -> expect summary type=html, no content element
	f1 := newAtomBaseFeed()
	item1 := newAtomBaseItem()
	item1.Description = "<p>Summary</p>"
	f1.Add(item1)
	xml1, err := f1.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom failed: %v", err)
	}
	var doc1 atomFeedDoc
	if err := xml.Unmarshal([]byte(xml1), &doc1); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	e1 := doc1.Entries[0]
	if e1.Summary == nil || e1.Summary.Type != "html" || !strings.Contains(e1.Summary.Text, "Summary") {
		t.Errorf("expected summary type=html from description")
	}
	if e1.Content != nil {
		t.Errorf("did not expect content element when only description is set")
	}

	// Content HTML -> expect content type=html
	f2 := newAtomBaseFeed()
	item2 := newAtomBaseItem()
	item2.Content = "<p>Body</p>"
	f2.Add(item2)
	xml2, err := f2.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom failed: %v", err)
	}
	var doc2 atomFeedDoc
	if err := xml.Unmarshal([]byte(xml2), &doc2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	e2 := doc2.Entries[0]
	if e2.Content == nil || e2.Content.Type != "html" || !strings.Contains(e2.Content.Text, "Body") {
		t.Errorf("expected content type=html when Content is provided")
	}
}

func TestAtomAutoIdGenerationTagURI(t *testing.T) {
	// When Item.ID is empty but Link+Date exist, expect tag: URI
	f := newAtomBaseFeed()
	item := newAtomBaseItem()
	item.ID = ""
	f.Add(item)
	xmlStr, err := f.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom failed: %v", err)
	}
	var doc atomFeedDoc
	if err := xml.Unmarshal([]byte(xmlStr), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	id := doc.Entries[0].Id
	if !strings.HasPrefix(id, "tag:") {
		t.Errorf("expected tag: URI auto-generated for entry id, got %q", id)
	}
}

// Standard requirement per RFC 4287 4.2.1: A feed must contain an author,
// unless all entries contain an author. This library currently does not enforce
// that constraint, so this test encodes the rule and may fail until enforced.
func TestAtomAuthorRequirementPerSpec(t *testing.T) {
	f := newAtomBaseFeed()
	// no feed.Author
	item := newAtomBaseItem()
	// no item.Author
	f.Add(item)
	xmlStr, err := f.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom failed: %v", err)
	}
	var doc atomFeedDoc
	if err := xml.Unmarshal([]byte(xmlStr), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	feedHasAuthor := doc.Author != nil && (doc.Author.Name != "" || doc.Author.Email != "")
	entriesHaveAuthors := true
	for _, e := range doc.Entries {
		if e.Author == nil || (e.Author.Name == "" && e.Author.Email == "") {
			entriesHaveAuthors = false
			break
		}
	}
	if !feedHasAuthor && !entriesHaveAuthors {
		t.Errorf("Atom spec: feed must have author or each entry must have an author (RFC 4287 4.2.1). Neither present.")
	}
}

func TestAtomDoesNotIncludePSPFields(t *testing.T) {
	feed := newAtomBaseFeed()
	item := newAtomBaseItem()
	feed.Add(item)

	// Configure PSP-only fields
	explicit := false
	feed.AtomSelfHref = "https://example.com/podcast.rss"
	feed.ItunesImageHref = "https://example.com/artwork.jpg"
	feed.ItunesExplicit = &explicit
	feed.ItunesType = "serial"
	feed.ItunesCategories = append(feed.ItunesCategories, &gofeedx.ItunesCategory{Text: "News"})
	feed.PodcastGuid = "a-guid"
	feed.PodcastFunding = &gofeedx.PodcastFunding{Url: "https://example.com/fund", Text: "Fund us"}
	feed.PodcastTXT = &gofeedx.PodcastTXT{Purpose: "verify", Value: "token"}

	ep := 2
	item.ItunesEpisode = &ep
	item.ItunesEpisodeType = "trailer"
	item.Transcripts = []gofeedx.PSPTranscript{{Url: "https://example.com/t2.vtt", Type: "text/vtt"}}

	// Serialize as Atom
	xmlStr, err := feed.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom failed: %v", err)
	}

	// Assert PSP namespaces/elements DO NOT appear in Atom
	if strings.Contains(xmlStr, `xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"`) ||
		strings.Contains(xmlStr, "<itunes:") {
		t.Errorf("unexpected itunes namespace/elements in Atom output")
	}
	if strings.Contains(xmlStr, `xmlns:podcast="https://podcastindex.org/namespace/1.0"`) ||
		strings.Contains(xmlStr, "<podcast:") {
		t.Errorf("unexpected podcast namespace/elements in Atom output")
	}
	// Atom output expected to have its default namespace only
	if strings.Contains(xmlStr, `xmlns:atom="http://www.w3.org/2005/Atom"`) {
		// our Atom writer sets default xmlns, not a prefix; presence of xmlns:atom would be suspicious
		t.Errorf("unexpected xmlns:atom prefix in Atom output (should use default xmlns)")
	}
}

func TestAtomExtensionNodesAllowed(t *testing.T) {
	feed := newAtomBaseFeed()
	item := newAtomBaseItem()
	feed.Add(item)

	// Add PSP-like elements via ExtensionNode (allowed exception)
	feed.Extensions = []gofeedx.ExtensionNode{
		{Name: "podcast:funding", Attrs: map[string]string{"url": "https://example.com/fund"}, Text: "Support"},
	}
	item.Extensions = []gofeedx.ExtensionNode{
		{Name: "itunes:image", Attrs: map[string]string{"href": "https://example.com/cover.jpg"}},
	}

	atom, err := feed.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom failed: %v", err)
	}
	if !strings.Contains(atom, "<podcast:funding") || !strings.Contains(atom, `url="https://example.com/fund"`) {
		t.Errorf("expected podcast:funding extension in Atom output")
	}
	if !strings.Contains(atom, "<itunes:image") || !strings.Contains(atom, `href="https://example.com/cover.jpg"`) {
		t.Errorf("expected itunes:image extension in Atom item output")
	}
}
