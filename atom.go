package gofeedx

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const atomNS = "http://www.w3.org/2005/Atom"

type AtomPerson struct {
	Name  string `xml:"name,omitempty"`
	Uri   string `xml:"uri,omitempty"`
	Email string `xml:"email,omitempty"`
}

type AtomSummary struct {
	XMLName xml.Name `xml:"summary"`
	Content string   `xml:",chardata"`
	Type    string   `xml:"type,attr"`
}

type AtomContent struct {
	XMLName xml.Name `xml:"content"`
	Content string   `xml:",chardata"`
	Type    string   `xml:"type,attr"`
}

type AtomAuthor struct {
	XMLName xml.Name `xml:"author"`
	AtomPerson
}

type AtomContributor struct {
	XMLName xml.Name `xml:"contributor"`
	AtomPerson
}

type AtomLink struct {
	XMLName xml.Name `xml:"link"`
	Href    string   `xml:"href,attr"`
	Rel     string   `xml:"rel,attr,omitempty"`
	Type    string   `xml:"type,attr,omitempty"`
	Length  string   `xml:"length,attr,omitempty"`
}

type AtomEntry struct {
	*AtomEntryExtension
	Title     string       `xml:"title"` // required
	Links     []AtomLink   // required if no child 'content' elements
	Source    string       `xml:"source,omitempty"`
	Author    *AtomAuthor  // required if feed lacks an author
	Summary   *AtomSummary // required if content has src or content is base64
	Content   *AtomContent
	Id        string `xml:"id"`      // required
	Updated   string `xml:"updated"` // required
	Published string `xml:"published,omitempty"`
}

type AtomEntryExtension struct {
	XMLName     xml.Name `xml:"entry"`
	Xmlns       string   `xml:"xmlns,attr,omitempty"`
	Category    string   `xml:"category,omitempty"`
	Rights      string   `xml:"rights,omitempty"`
	Contributor *AtomContributor
	Extra       []ExtensionNode `xml:",any"` // custom extension nodes
}

type AtomFeed struct {
	*AtomFeedExtension
	Title    string `xml:"title"` // required
	Link     *AtomLink
	Subtitle string       `xml:"subtitle,omitempty"`
	Author   *AtomAuthor  `xml:"author,omitempty"`
	Updated  string       `xml:"updated"` // required
	Id       string       `xml:"id"`      // required
	Entries  []*AtomEntry `xml:"entry"`
	Category string       `xml:"category,omitempty"`
	Rights   string       `xml:"rights,omitempty"` // copyright used
	Logo     string       `xml:"logo,omitempty"`
}

type AtomFeedExtension struct {
	XMLName     xml.Name `xml:"feed"`
	Xmlns       string   `xml:"xmlns,attr"`
	Icon        string   `xml:"icon,omitempty"`
	Contributor *AtomContributor
	Extra       []ExtensionNode `xml:",any"` // custom extension nodes
}

type Atom struct {
	*Feed
}




// FeedXml returns an XML-Ready object for an Atom object
func (a *Atom) FeedXml() interface{} {
	return a.AtomFeed()
}

// FeedXml returns an XML-ready object for an AtomFeed object
func (a *AtomFeed) FeedXml() interface{} {
	return a
}

func (a *Atom) AtomFeed() *AtomFeed {
	updated := anyTimeFormat(time.RFC3339, a.Updated, a.Created)
	link := a.Link
	if link == nil {
		link = &Link{}
	}
	feed := &AtomFeed{
		AtomFeedExtension: &AtomFeedExtension{
			Xmlns: atomNS,
		},
		Title:    a.Title,
		Link:     &AtomLink{Href: link.Href, Rel: "alternate"},
		Subtitle: a.Description,
		Id:       firstNonEmpty(a.ID, link.Href),
		Updated:  updated,
		Rights:   a.Copyright,
	}

	// Map generic image to Atom logo/icon when available
	if a.Image != nil && a.Image.Url != "" {
		if feed.Logo == "" {
			feed.Logo = a.Image.Url
		}
		if feed.Icon == "" {
			feed.Icon = a.Image.Url
		}
	}

	if a.Author != nil {
		feed.Author = &AtomAuthor{AtomPerson: AtomPerson{Name: a.Author.Name, Email: a.Author.Email}}
	}

	// Map generic categories: Atom uses only the first top-level category when present
	if len(a.Categories) > 0 && a.Categories[0] != nil && a.Categories[0].Text != "" {
		feed.Category = a.Categories[0].Text
	}

	for _, e := range a.Items {
		feed.Entries = append(feed.Entries, newAtomEntry(e))
	}

	// Ensure Atom author requirement (RFC 4287 4.2.1):
	// A feed must contain an author, unless all entries contain an author.
	if feed.Author == nil {
		allEntriesHaveAuthors := true
		for _, it := range a.Items {
			if it.Author == nil || (it.Author.Name == "" && it.Author.Email == "") {
				allEntriesHaveAuthors = false
				break
			}
		}
		if !allEntriesHaveAuthors {
			feed.Author = &AtomAuthor{AtomPerson: AtomPerson{Name: "unknown"}}
		}
	}

	// Custom channel/feed extensions
	if len(a.Extensions) > 0 {
		feed.Extra = append(feed.Extra, a.Extensions...)
	}
	return feed
}

func newAtomEntry(i *Item) *AtomEntry {
	id := i.ID
	link := i.Link
	if link == nil {
		link = &Link{}
	}
	if len(id) == 0 {
		// Create a tag URI if we have a URL and any timestamp, else fallback to UUID URN
		if len(link.Href) > 0 && (!i.Created.IsZero() || !i.Updated.IsZero()) {
			dateStr := anyTimeFormat("2006-01-02", i.Updated, i.Created)
			host, path := link.Href, "/"
			if u, err := url.Parse(link.Href); err == nil {
				host, path = u.Host, u.Path
			}
			id = fmt.Sprintf("tag:%s,%s:%s", host, dateStr, path)
		} else {
			id = "urn:uuid:" + MustUUIDv4().String()
		}
	}

	var name, email string
	if i.Author != nil {
		name, email = i.Author.Name, i.Author.Email
	}

	linkRel := "alternate"
	x := &AtomEntry{
		Title:   i.Title,
		Links:   []AtomLink{{Href: link.Href, Rel: linkRel}},
		Id:      id,
		Updated: anyTimeFormat(time.RFC3339, i.Updated, i.Created),
	}
	// Published maps to item Created timestamp when available
	if !i.Created.IsZero() {
		x.Published = i.Created.Format(time.RFC3339)
	}

	// Summary from description (assume html)
	if len(i.Description) > 0 {
		x.Summary = &AtomSummary{Content: i.Description, Type: "html"}
	}

	// Content as HTML
	if len(i.Content) > 0 {
		x.Content = &AtomContent{Content: i.Content, Type: "html"}
	}

	// Enclosure if present and not already the main link
	if i.Enclosure != nil && linkRel != "enclosure" {
		x.Links = append(x.Links, AtomLink{Href: i.Enclosure.Url, Rel: "enclosure", Type: i.Enclosure.Type, Length: ""})
	}

	// Related/source link if provided
	if i.Source != nil && i.Source.Href != "" {
		x.Links = append(x.Links, AtomLink{Href: i.Source.Href, Rel: "related"})
	}

	if len(name) > 0 || len(email) > 0 {
		x.Author = &AtomAuthor{AtomPerson: AtomPerson{Name: name, Email: email}}
	}

	// Custom item/entry extensions
	if len(i.Extensions) > 0 {
		if x.AtomEntryExtension == nil {
			x.AtomEntryExtension = &AtomEntryExtension{
				Xmlns: atomNS,
			}
		}
		x.Extra = append(x.Extra, i.Extensions...)
	}
	return x
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ValidateAtom enforces Atom 1.0 (RFC 4287) essentials on the generic Feed.
func ValidateAtom(f *Feed) error {
	// Feed-level required: title, updated (from Updated or Created), id (from ID or Link.Href)
	if strings.TrimSpace(f.Title) == "" {
		return errors.New("atom: feed title required")
	}
	if f.Updated.IsZero() && f.Created.IsZero() {
		return errors.New("atom: feed updated timestamp required (use Feed.Updated or Feed.Created)")
	}
	if strings.TrimSpace(f.ID) == "" && (f.Link == nil || strings.TrimSpace(f.Link.Href) == "") {
		return errors.New("atom: feed id required (set Feed.ID or Link.Href)")
	}
	// At least one entry
	if len(f.Items) == 0 {
		return errors.New("atom: at least one entry required")
	}
	// Entry-level: title and updated (from Updated or Created)
	for i, it := range f.Items {
		if strings.TrimSpace(it.Title) == "" {
			return fmt.Errorf("atom: entry[%d] title required", i)
		}
		if it.Updated.IsZero() && it.Created.IsZero() {
			return fmt.Errorf("atom: entry[%d] updated timestamp required (use Item.Updated or Item.Created)", i)
		}
	}
	// Author requirement (RFC 4287 4.2.1): feed must have author unless all entries have one
	if f.Author == nil || (strings.TrimSpace(f.Author.Name) == "" && strings.TrimSpace(f.Author.Email) == "") {
		allEntriesHaveAuthors := true
		for _, it := range f.Items {
			if it.Author == nil || (strings.TrimSpace(it.Author.Name) == "" && strings.TrimSpace(it.Author.Email) == "") {
				allEntriesHaveAuthors = false
				break
			}
		}
		if !allEntriesHaveAuthors {
			return errors.New("atom: feed must contain an author or all entries must contain an author (RFC 4287 4.2.1)")
		}
	}
	return nil
}


