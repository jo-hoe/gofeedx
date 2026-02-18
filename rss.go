package gofeedx

// RSS 2.0 encoder (with optional content:encoded for HTML content)
import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"
)

// RssFeedXml is the <rss> root wrapper.
type RssFeedXml struct {
	XMLName          xml.Name `xml:"rss"`
	Version          string   `xml:"version,attr"`
	ContentNamespace string   `xml:"xmlns:content,attr,omitempty"`
	Channel          *RssFeed `xml:"channel"`
}

// RssContent holds HTML content in CDATA via content:encoded.
type RssContent struct {
	XMLName xml.Name `xml:"content:encoded"`
	Content string   `xml:",cdata"`
}

type RssImage struct {
	XMLName xml.Name `xml:"image"`
	Url     string   `xml:"url"`
	Title   string   `xml:"title"`
	Link    string   `xml:"link"`
	Width   int      `xml:"width,omitempty"`
	Height  int      `xml:"height,omitempty"`
}

type RssEnclosure struct {
	XMLName xml.Name `xml:"enclosure"`
	Url     string   `xml:"url,attr"`
	Length  string   `xml:"length,attr"`
	Type    string   `xml:"type,attr"`
}

type RssGuid struct {
	XMLName     xml.Name `xml:"guid"`
	ID          string   `xml:",chardata"`
	IsPermaLink string   `xml:"isPermaLink,attr,omitempty"` // "true", "false", or omitted
}

type RssItem struct {
	XMLName     xml.Name     `xml:"item"`
	Title       string       `xml:"title"`       // required
	Link        string       `xml:"link"`        // required by spec, but often omitted by feeds
	Description string       `xml:"description"` // required by spec; we include if provided
	Content     *RssContent  `xml:"content:encoded,omitempty"`
	Author      string       `xml:"author,omitempty"`
	Category    string       `xml:"category,omitempty"`
	Comments    string       `xml:"comments,omitempty"`
	Enclosure   *RssEnclosure
	Guid        *RssGuid
	PubDate     string    `xml:"pubDate,omitempty"`
	Source      string    `xml:"source,omitempty"`
	Extra       []ExtensionNode `xml:",any"` // custom nodes at item scope
}

type RssFeed struct {
	XMLName       xml.Name  `xml:"channel"`
	Title         string    `xml:"title"`       // required
	Link          string    `xml:"link"`        // required
	Description   string    `xml:"description"` // required
	Language      string    `xml:"language,omitempty"`
	Copyright     string    `xml:"copyright,omitempty"`
	ManagingEditor string   `xml:"managingEditor,omitempty"`
	WebMaster     string    `xml:"webMaster,omitempty"`
	PubDate       string    `xml:"pubDate,omitempty"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	Category      string    `xml:"category,omitempty"`
	Generator     string    `xml:"generator,omitempty"`
	Docs          string    `xml:"docs,omitempty"`
	Cloud         string    `xml:"cloud,omitempty"`
	Ttl           int       `xml:"ttl,omitempty"`
	Rating        string    `xml:"rating,omitempty"`
	SkipHours     string    `xml:"skipHours,omitempty"`
	SkipDays      string    `xml:"skipDays,omitempty"`
	Image         *RssImage `xml:"image,omitempty"`
	Items         []*RssItem `xml:"item"`
	Extra         []ExtensionNode `xml:",any"` // custom nodes at channel scope
}

// Rss is a wrapper to marshal a Feed as RSS 2.0.
type Rss struct {
	*Feed
}

// ToRSS creates an RSS 2.0 representation of this feed as string.
func (f *Feed) ToRSS() (string, error) {
	return ToXML(&Rss{f})
}

// WriteRSS writes an RSS 2.0 representation of this feed to the writer.
func (f *Feed) WriteRSS(w io.Writer) error {
	return WriteXML(&Rss{f}, w)
}

// FeedXml returns an XML-Ready object for an Rss object.
func (r *Rss) FeedXml() interface{} {
	return r.RssFeed().FeedXml()
}

// RssFeed builds the channel structure from the generic Feed.
func (r *Rss) RssFeed() *RssFeed {
	pub := anyTimeFormat(time.RFC1123Z, r.Created, r.Updated)
	build := anyTimeFormat(time.RFC1123Z, r.Updated)
	author := ""
	if r.Author != nil {
		author = r.Author.Email
		if len(r.Author.Name) > 0 {
			author = fmt.Sprintf("%s (%s)", r.Author.Email, r.Author.Name)
		}
	}

	var image *RssImage
	if r.Image != nil {
		image = &RssImage{
			Url:    r.Image.Url,
			Title:  r.Image.Title,
			Link:   r.Image.Link,
			Width:  r.Image.Width,
			Height: r.Image.Height,
		}
	}

	var href string
	if r.Link != nil {
		href = r.Link.Href
	}
	channel := &RssFeed{
		Title:          r.Title,
		Link:           href,
		Description:    r.Description,
		ManagingEditor: author,
		PubDate:        pub,
		LastBuildDate:  build,
		Copyright:      r.Copyright,
		Image:          image,
		Language:       r.Language,
	}

	// append items
	for _, it := range r.Items {
		channel.Items = append(channel.Items, newRssItem(it))
	}

	// append extensions
	if len(r.Extensions) > 0 {
		channel.Extra = append(channel.Extra, r.Extensions...)
	}
	return channel
}

// FeedXml returns an XML-ready object for an RssFeed object (wrapped with <rss>).
func (r *RssFeed) FeedXml() interface{} {
	// Only add content namespace if any item has content:encoded
	contentNS := ""
scan:
	for _, it := range r.Items {
		if it.Content != nil && it.Content.Content != "" {
			contentNS = "http://purl.org/rss/1.0/modules/content/"
			break scan
		}
	}
	return &RssFeedXml{
		Version:          "2.0",
		Channel:          r,
		ContentNamespace: contentNS,
	}
}

func newRssItem(i *Item) *RssItem {
	item := &RssItem{
		Title:       i.Title,
		Description: i.Description,
		PubDate:     anyTimeFormat(time.RFC1123Z, i.Created, i.Updated),
	}
	if i.ID != "" {
		item.Guid = &RssGuid{ID: i.ID, IsPermaLink: i.IsPermaLink}
	}
	if i.Link != nil {
		item.Link = i.Link.Href
	}
	if len(i.Content) > 0 {
		item.Content = &RssContent{Content: i.Content}
	}
	if i.Source != nil {
		item.Source = i.Source.Href
	}
	if i.Enclosure != nil && i.Enclosure.Type != "" && i.Enclosure.Url != "" && i.Enclosure.Length >= 0 {
		item.Enclosure = &RssEnclosure{
			Url:    i.Enclosure.Url,
			Type:   i.Enclosure.Type,
			Length: strconv.FormatInt(i.Enclosure.Length, 10),
		}
	}
	if i.Author != nil {
		item.Author = i.Author.Name
	}
	// append extensions
	if len(i.Extensions) > 0 {
		item.Extra = append(item.Extra, i.Extensions...)
	}
	return item
}