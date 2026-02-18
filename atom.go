package gofeedx

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
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
	XMLName     xml.Name `xml:"entry"`
	Xmlns       string   `xml:"xmlns,attr,omitempty"`
	Title       string   `xml:"title"`   // required
	Updated     string   `xml:"updated"` // required
	Id          string   `xml:"id"`      // required
	Category    string   `xml:"category,omitempty"`
	Content     *AtomContent
	Rights      string `xml:"rights,omitempty"`
	Source      string `xml:"source,omitempty"`
	Published   string `xml:"published,omitempty"`
	Contributor *AtomContributor
	Links       []AtomLink      // required if no child 'content' elements
	Summary     *AtomSummary    // required if content has src or content is base64
	Author      *AtomAuthor     // required if feed lacks an author
	Extra       []ExtensionNode `xml:",any"` // custom extension nodes
}

type AtomFeed struct {
	XMLName     xml.Name `xml:"feed"`
	Xmlns       string   `xml:"xmlns,attr"`
	Title       string   `xml:"title"`   // required
	Id          string   `xml:"id"`      // required
	Updated     string   `xml:"updated"` // required
	Category    string   `xml:"category,omitempty"`
	Icon        string   `xml:"icon,omitempty"`
	Logo        string   `xml:"logo,omitempty"`
	Rights      string   `xml:"rights,omitempty"` // copyright used
	Subtitle    string   `xml:"subtitle,omitempty"`
	Link        *AtomLink
	Author      *AtomAuthor `xml:"author,omitempty"`
	Contributor *AtomContributor
	Entries     []*AtomEntry    `xml:"entry"`
	Extra       []ExtensionNode `xml:",any"` // custom extension nodes
}

type Atom struct {
	*Feed
}

// ToAtom creates an Atom 1.0 representation of this feed as string.
func (f *Feed) ToAtom() (string, error) {
	return ToXML(&Atom{f})
}

// WriteAtom writes an Atom 1.0 representation of this feed to the writer.
func (f *Feed) WriteAtom(w io.Writer) error {
	return WriteXML(&Atom{f}, w)
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
		Xmlns:    atomNS,
		Title:    a.Title,
		Link:     &AtomLink{Href: link.Href, Rel: firstNonEmpty(link.Rel, "alternate"), Type: link.Type, Length: link.Length},
		Subtitle: a.Description,
		Id:       firstNonEmpty(a.ID, link.Href),
		Updated:  updated,
		Rights:   a.Copyright,
	}

	if a.Author != nil {
		feed.Author = &AtomAuthor{AtomPerson: AtomPerson{Name: a.Author.Name, Email: a.Author.Email}}
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

	linkRel := firstNonEmpty(link.Rel, "alternate")
	x := &AtomEntry{
		Title:   i.Title,
		Links:   []AtomLink{{Href: link.Href, Rel: linkRel, Type: link.Type, Length: link.Length}},
		Id:      id,
		Updated: anyTimeFormat(time.RFC3339, i.Updated, i.Created),
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

	if len(name) > 0 || len(email) > 0 {
		x.Author = &AtomAuthor{AtomPerson: AtomPerson{Name: name, Email: email}}
	}

	// Custom item/entry extensions
	if len(i.Extensions) > 0 {
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
