package gofeedx

import (
	"encoding/xml"
	"errors"
	"fmt"
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
	Title       string `xml:"title"` // required
	Links       []AtomLink
	Source      string      `xml:"source,omitempty"`
	Author      *AtomAuthor // required if feed lacks an author
	Summary     *AtomSummary
	Content     *AtomContent
	Id          string   `xml:"id"`      // required
	Updated     string   `xml:"updated"` // required
	Published   string   `xml:"published,omitempty"`
	XMLName     xml.Name `xml:"entry"`
	Xmlns       string   `xml:"xmlns,attr,omitempty"`
	Category    string   `xml:"category,omitempty"`
	Rights      string   `xml:"rights,omitempty"`
	Contributor *AtomContributor
	Extra       []ExtensionNode `xml:",any"` // custom extension nodes
}

type AtomFeed struct {
	Title       string `xml:"title"` // required
	Link        *AtomLink
	Subtitle    string       `xml:"subtitle,omitempty"`
	Author      *AtomAuthor  `xml:"author,omitempty"`
	Updated     string       `xml:"updated"` // required
	Id          string       `xml:"id"`      // required
	Entries     []*AtomEntry `xml:"entry"`
	Category    string       `xml:"category,omitempty"`
	Rights      string       `xml:"rights,omitempty"` // copyright used
	Logo        string       `xml:"logo,omitempty"`
	XMLName     xml.Name     `xml:"feed"`
	Xmlns       string       `xml:"xmlns,attr"`
	Icon        string       `xml:"icon,omitempty"`
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

// Helpers to reduce cyclomatic complexity

func atomFeedBaseFromFeed(a *Atom) *AtomFeed {
	updated := anyTimeFormat(time.RFC3339, a.Updated, a.Created)
	link := a.Link
	if link == nil {
		link = &Link{}
	}
	return &AtomFeed{
		Xmlns:    atomNS,
		Title:    a.Title,
		Link:     &AtomLink{Href: link.Href, Rel: "alternate"},
		Subtitle: a.Description,
		Id:       firstNonEmpty(a.ID, link.Href),
		Updated:  updated,
		Rights:   a.Copyright,
	}
}

func applyAtomImage(feed *AtomFeed, img *Image) {
	if img == nil || img.Url == "" {
		return
	}
	if feed.Logo == "" {
		feed.Logo = img.Url
	}
	if feed.Icon == "" {
		feed.Icon = img.Url
	}
}

func setAtomAuthorFromFeed(feed *AtomFeed, author *Author) {
	if author == nil {
		return
	}
	feed.Author = &AtomAuthor{AtomPerson: AtomPerson{Name: author.Name, Email: author.Email}}
}

func setFirstCategory(feed *AtomFeed, cats []*Category) {
	if len(cats) > 0 && cats[0] != nil && cats[0].Text != "" {
		feed.Category = cats[0].Text
	}
}

func addEntriesToFeed(feed *AtomFeed, items []*Item) {
	for _, e := range items {
		feed.Entries = append(feed.Entries, newAtomEntry(e))
	}
}

func ensureAtomAuthorRequirement(feed *AtomFeed, items []*Item) {
	if feed.Author != nil {
		return
	}
	allEntriesHaveAuthors := true
	for _, it := range items {
		if it.Author == nil || (it.Author.Name == "" && it.Author.Email == "") {
			allEntriesHaveAuthors = false
			break
		}
	}
	if !allEntriesHaveAuthors {
		feed.Author = &AtomAuthor{AtomPerson: AtomPerson{Name: "unknown"}}
	}
}

func mapAtomFeedExtensions(feed *AtomFeed, exts []ExtensionNode) {
	if len(exts) == 0 {
		return
	}
	var extras []ExtensionNode
	for _, n := range exts {
		name := strings.TrimSpace(strings.ToLower(n.Name))
		switch name {
		case "_atom:icon":
			if s := strings.TrimSpace(n.Text); s != "" {
				feed.Icon = s
			} else {
				extras = append(extras, n)
			}
		case "_atom:logo":
			if s := strings.TrimSpace(n.Text); s != "" {
				feed.Logo = s
			} else {
				extras = append(extras, n)
			}
		case "_atom:rights":
			if s := strings.TrimSpace(n.Text); s != "" {
				feed.Rights = s
			} else {
				extras = append(extras, n)
			}
		case "_atom:contributor":
			var ap AtomPerson
			if n.Attrs != nil {
				ap.Name = strings.TrimSpace(n.Attrs["name"])
				ap.Email = strings.TrimSpace(n.Attrs["email"])
				ap.Uri = strings.TrimSpace(n.Attrs["uri"])
			}
			if ap.Name != "" || ap.Email != "" || ap.Uri != "" {
				feed.Contributor = &AtomContributor{AtomPerson: ap}
			} else {
				extras = append(extras, n)
			}
		case "_atom:link":
			var l AtomLink
			if n.Attrs != nil {
				l.Href = strings.TrimSpace(n.Attrs["href"])
				l.Rel = strings.TrimSpace(n.Attrs["rel"])
				l.Type = strings.TrimSpace(n.Attrs["type"])
				l.Length = strings.TrimSpace(n.Attrs["length"])
			}
			if l.Href != "" {
				feed.Link = &l
			} else {
				extras = append(extras, n)
			}
		default:
			extras = append(extras, n)
		}
	}
	if len(extras) > 0 {
		feed.Extra = append(feed.Extra, extras...)
	}
}

func (a *Atom) AtomFeed() *AtomFeed {
	feed := atomFeedBaseFromFeed(a)
	applyAtomImage(feed, a.Image)
	setAtomAuthorFromFeed(feed, a.Author)
	setFirstCategory(feed, a.Categories)
	addEntriesToFeed(feed, a.Items)
	ensureAtomAuthorRequirement(feed, a.Items)
	mapAtomFeedExtensions(feed, a.Extensions)
	return feed
}

func atomEntryBase(i *Item) *AtomEntry {
	id := strings.TrimSpace(i.ID)
	if id == "" {
		id = fallbackItemGuid(i)
	}
	link := i.Link
	if link == nil {
		link = &Link{}
	}
	x := &AtomEntry{
		Title:   i.Title,
		Links:   []AtomLink{{Href: link.Href, Rel: "alternate"}},
		Id:      id,
		Updated: anyTimeFormat(time.RFC3339, i.Updated, i.Created),
		Xmlns:   atomNS,
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
	// Author
	if i.Author != nil && (i.Author.Name != "" || i.Author.Email != "") {
		x.Author = &AtomAuthor{AtomPerson: AtomPerson{Name: i.Author.Name, Email: i.Author.Email}}
	}
	return x
}

func addEnclosureAndRelatedLinks(x *AtomEntry, i *Item) {
	// Enclosure if present
	if i.Enclosure != nil {
		x.Links = append(x.Links, AtomLink{Href: i.Enclosure.Url, Rel: "enclosure", Type: i.Enclosure.Type, Length: ""})
	}
	// Related/source link if provided
	if i.Source != nil && i.Source.Href != "" {
		x.Links = append(x.Links, AtomLink{Href: i.Source.Href, Rel: "related"})
	}
}

func mapAtomEntryExtensions(x *AtomEntry, exts []ExtensionNode) {
	if len(exts) == 0 {
		return
	}
	var extras []ExtensionNode
	for _, n := range exts {
		name := strings.TrimSpace(strings.ToLower(n.Name))
		switch name {
		case "_atom:category":
			if s := strings.TrimSpace(n.Text); s != "" {
				x.Category = s
			} else {
				extras = append(extras, n)
			}
		case "_atom:rights":
			if s := strings.TrimSpace(n.Text); s != "" {
				x.Rights = s
			} else {
				extras = append(extras, n)
			}
		case "_atom:contributor":
			var ap AtomPerson
			if n.Attrs != nil {
				ap.Name = strings.TrimSpace(n.Attrs["name"])
				ap.Email = strings.TrimSpace(n.Attrs["email"])
				ap.Uri = strings.TrimSpace(n.Attrs["uri"])
			}
			if ap.Name != "" || ap.Email != "" || ap.Uri != "" {
				x.Contributor = &AtomContributor{AtomPerson: ap}
			} else {
				extras = append(extras, n)
			}
		case "_atom:link":
			var l AtomLink
			if n.Attrs != nil {
				l.Href = strings.TrimSpace(n.Attrs["href"])
				l.Rel = strings.TrimSpace(n.Attrs["rel"])
				l.Type = strings.TrimSpace(n.Attrs["type"])
				l.Length = strings.TrimSpace(n.Attrs["length"])
			}
			if l.Href != "" {
				x.Links = append(x.Links, l)
			} else {
				extras = append(extras, n)
			}
		case "_atom:source":
			if s := strings.TrimSpace(n.Text); s != "" {
				x.Source = s
			} else {
				extras = append(extras, n)
			}
		default:
			extras = append(extras, n)
		}
	}
	if len(extras) > 0 {
		x.Extra = append(x.Extra, extras...)
	}
}

func newAtomEntry(i *Item) *AtomEntry {
	x := atomEntryBase(i)
	addEnclosureAndRelatedLinks(x, i)
	mapAtomEntryExtensions(x, i.Extensions)
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

// Atom-specific builder helpers implemented here without touching generic files.
// Feed-level helpers:

// WithAtomIcon sets feed-level icon override.
func (b *FeedBuilder) WithAtomIcon(url string) *FeedBuilder {
	url = strings.TrimSpace(url)
	if url == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_atom:icon", Text: url})
}

// WithAtomLogo sets feed-level logo override.
func (b *FeedBuilder) WithAtomLogo(url string) *FeedBuilder {
	url = strings.TrimSpace(url)
	if url == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_atom:logo", Text: url})
}

// WithAtomRights sets feed-level rights text (copyright override).
func (b *FeedBuilder) WithAtomRights(text string) *FeedBuilder {
	text = strings.TrimSpace(text)
	if text == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_atom:rights", Text: text})
}

// WithAtomContributor sets a feed-level contributor.
func (b *FeedBuilder) WithAtomContributor(name, email, uri string) *FeedBuilder {
	attrs := map[string]string{}
	if s := strings.TrimSpace(name); s != "" {
		attrs["name"] = s
	}
	if s := strings.TrimSpace(email); s != "" {
		attrs["email"] = s
	}
	if s := strings.TrimSpace(uri); s != "" {
		attrs["uri"] = s
	}
	if len(attrs) == 0 {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_atom:contributor", Attrs: attrs})
}

// WithAtomFeedLink overrides/adds the primary feed link with attributes.
func (b *FeedBuilder) WithAtomFeedLink(href, rel, typ, length string) *FeedBuilder {
	attrs := map[string]string{}
	if s := strings.TrimSpace(href); s != "" {
		attrs["href"] = s
	}
	if s := strings.TrimSpace(rel); s != "" {
		attrs["rel"] = s
	}
	if s := strings.TrimSpace(typ); s != "" {
		attrs["type"] = s
	}
	if s := strings.TrimSpace(length); s != "" {
		attrs["length"] = s
	}
	if len(attrs) == 0 {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_atom:link", Attrs: attrs})
}

// Item-level helpers:

// WithAtomCategory sets entry category.
func (b *ItemBuilder) WithAtomCategory(text string) *ItemBuilder {
	text = strings.TrimSpace(text)
	if text == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_atom:category", Text: text})
}

// WithAtomRights sets entry rights.
func (b *ItemBuilder) WithAtomRights(text string) *ItemBuilder {
	text = strings.TrimSpace(text)
	if text == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_atom:rights", Text: text})
}

// WithAtomContributor sets entry-level contributor.
func (b *ItemBuilder) WithAtomContributor(name, email, uri string) *ItemBuilder {
	attrs := map[string]string{}
	if s := strings.TrimSpace(name); s != "" {
		attrs["name"] = s
	}
	if s := strings.TrimSpace(email); s != "" {
		attrs["email"] = s
	}
	if s := strings.TrimSpace(uri); s != "" {
		attrs["uri"] = s
	}
	if len(attrs) == 0 {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_atom:contributor", Attrs: attrs})
}

// WithAtomLink appends an additional link to the entry.
func (b *ItemBuilder) WithAtomLink(href, rel, typ, length string) *ItemBuilder {
	attrs := map[string]string{}
	if s := strings.TrimSpace(href); s != "" {
		attrs["href"] = s
	}
	if s := strings.TrimSpace(rel); s != "" {
		attrs["rel"] = s
	}
	if s := strings.TrimSpace(typ); s != "" {
		attrs["type"] = s
	}
	if s := strings.TrimSpace(length); s != "" {
		attrs["length"] = s
	}
	if len(attrs) == 0 {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_atom:link", Attrs: attrs})
}

// WithAtomSource sets the entry source.
func (b *ItemBuilder) WithAtomSource(src string) *ItemBuilder {
	src = strings.TrimSpace(src)
	if src == "" {
		return b
	}
	return b.WithExtensions(ExtensionNode{Name: "_atom:source", Text: src})
}