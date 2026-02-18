package gofeedx

// RSS 2.0 encoder (with optional content:encoded for HTML content)
import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
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
	XMLName     xml.Name    `xml:"item"`
	Title       string      `xml:"title"`       // required
	Link        string      `xml:"link"`        // required by spec, but often omitted by feeds
	Description string      `xml:"description"` // required by spec; we include if provided
	Content     *RssContent `xml:"content:encoded,omitempty"`
	Author      string      `xml:"author,omitempty"`
	Category    string      `xml:"category,omitempty"`
	Comments    string      `xml:"comments,omitempty"`
	Enclosure   *RssEnclosure
	Guid        *RssGuid
	PubDate     string          `xml:"pubDate,omitempty"`
	Source      string          `xml:"source,omitempty"`
	Extra       []ExtensionNode `xml:",any"` // custom nodes at item scope
}

type RssFeed struct {
	XMLName        xml.Name        `xml:"channel"`
	Title          string          `xml:"title"`       // required
	Link           string          `xml:"link"`        // required
	Description    string          `xml:"description"` // required
	Language       string          `xml:"language,omitempty"`
	Copyright      string          `xml:"copyright,omitempty"`
	ManagingEditor string          `xml:"managingEditor,omitempty"`
	WebMaster      string          `xml:"webMaster,omitempty"`
	PubDate        string          `xml:"pubDate,omitempty"`
	LastBuildDate  string          `xml:"lastBuildDate,omitempty"`
	Category       string          `xml:"category,omitempty"`
	Generator      string          `xml:"generator,omitempty"`
	Docs           string          `xml:"docs,omitempty"`
	Cloud          string          `xml:"cloud,omitempty"`
	Ttl            int             `xml:"ttl,omitempty"`
	Rating         string          `xml:"rating,omitempty"`
	SkipHours      string          `xml:"skipHours,omitempty"`
	SkipDays       string          `xml:"skipDays,omitempty"`
	Image          *RssImage       `xml:"image,omitempty"`
	Items          []*RssItem      `xml:"item"`
	Extra          []ExtensionNode `xml:",any"` // custom nodes at channel scope
}

// Rss is a wrapper to marshal a Feed as RSS 2.0.
type Rss struct {
	*Feed
}

/*
ToRSSString creates an RSS 2.0 representation of this feed as a string.
Use ToRSSFeed() if you need the structured root object for further processing.
*/
func (f *Feed) ToRSSString() (string, error) {
	return ToXML(&Rss{f})
}

/*
ToRSSFeed returns the RSS 2.0 root struct for this feed.
*/
func (f *Feed) ToRSSFeed() (*RssFeedXml, error) {
	r := &Rss{f}
	rf := r.RssFeed()
	root, _ := rf.FeedXml().(*RssFeedXml)
	return root, nil
}

/*
WriteRSS writes an RSS 2.0 representation of this feed to the writer.
*/
func (f *Feed) WriteRSS(w io.Writer) error {
	return WriteXML(&Rss{f}, w)
}

// ==========================
// RSS encoder functional options (moved from rss_opts.go)
// ==========================

// rssConfig holds optional encoder-specific knobs for RSS output.
type rssConfig struct {
	ImageWidth       int
	ImageHeight      int
	TTL              int
	CategoryOverride string
}

// RSSOption is a functional option to configure RSS encoding.
type RSSOption func(*rssConfig)

// WithRSSImageSize sets <image><width> and <image><height>.
func WithRSSImageSize(width, height int) RSSOption {
	return func(c *rssConfig) {
		c.ImageWidth = width
		c.ImageHeight = height
	}
}

// WithRSSTTL sets the channel-level <ttl> in minutes.
func WithRSSTTL(ttl int) RSSOption {
	return func(c *rssConfig) { c.TTL = ttl }
}

// WithRSSChannelCategory overrides the single <category> string for the channel.
func WithRSSChannelCategory(cat string) RSSOption {
	return func(c *rssConfig) { c.CategoryOverride = cat }
}

/*
ToRSSStringOpts creates an RSS 2.0 representation of this feed as a string,
using optional encoder-specific options.
*/
func (f *Feed) ToRSSStringOpts(opts ...RSSOption) (string, error) {
	cfg := &rssConfig{}
	for _, o := range opts {
		o(cfg)
	}
	channel := buildRssFeedWithOpts(f, cfg)
	return ToXML(channel)
}

/*
WriteRSSOpts writes an RSS 2.0 representation of this feed to the writer,
using optional encoder-specific options.
*/
func (f *Feed) WriteRSSOpts(w io.Writer, opts ...RSSOption) error {
	cfg := &rssConfig{}
	for _, o := range opts {
		o(cfg)
	}
	channel := buildRssFeedWithOpts(f, cfg)
	return WriteXML(channel, w)
}

/*
ToRSSFeedOpts returns the RSS 2.0 root struct for this feed,
using optional encoder-specific options.
*/
func (f *Feed) ToRSSFeedOpts(opts ...RSSOption) (*RssFeedXml, error) {
	cfg := &rssConfig{}
	for _, o := range opts {
		o(cfg)
	}
	channel := buildRssFeedWithOpts(f, cfg)
	root, _ := channel.FeedXml().(*RssFeedXml)
	return root, nil
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
			Url:   r.Image.Url,
			Title: r.Image.Title,
			Link:  r.Image.Link,
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

	// Map generic categories: RSS uses single category string; use first if present
	if len(r.Categories) > 0 && r.Categories[0] != nil && r.Categories[0].Text != "" {
		channel.Category = r.Categories[0].Text
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

func buildRssFeedWithOpts(r *Feed, cfg *rssConfig) *RssFeed {
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
			Width:  cfg.ImageWidth,
			Height: cfg.ImageHeight,
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
		Ttl:            cfg.TTL,
	}

	// Category override or generic mapping
	if cfg.CategoryOverride != "" {
		channel.Category = cfg.CategoryOverride
	} else if len(r.Categories) > 0 && r.Categories[0] != nil && r.Categories[0].Text != "" {
		channel.Category = r.Categories[0].Text
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
	if i.Enclosure != nil && i.Enclosure.Type != "" && i.Enclosure.Url != "" && i.Enclosure.Length > 0 {
		item.Enclosure = &RssEnclosure{
			Url:    i.Enclosure.Url,
			Type:   i.Enclosure.Type,
			Length: strconv.FormatInt(i.Enclosure.Length, 10),
		}
	}
	if i.Author != nil {
		author := i.Author.Email
		if i.Author.Name != "" {
			author = fmt.Sprintf("%s (%s)", i.Author.Email, i.Author.Name)
		}
		item.Author = author
	}
	// append extensions
	if len(i.Extensions) > 0 {
		item.Extra = append(item.Extra, i.Extensions...)
	}
	return item
}

// ValidateRSS enforces basic RSS 2.0.1 requirements on the generic Feed.
func (f *Feed) ValidateRSS() error {
	// Channel-level required fields per RSS 2.0.1
	if strings.TrimSpace(f.Title) == "" {
		return errors.New("rss: channel title required")
	}
	if f.Link == nil || strings.TrimSpace(f.Link.Href) == "" {
		return errors.New("rss: channel link required")
	}
	if strings.TrimSpace(f.Description) == "" {
		return errors.New("rss: channel description required")
	}
	// Items
	if len(f.Items) == 0 {
		return errors.New("rss: at least one item required")
	}
	for i, it := range f.Items {
		// An item should have at least a title or a description
		if strings.TrimSpace(it.Title) == "" && strings.TrimSpace(it.Description) == "" {
			return fmt.Errorf("rss: item[%d] must include a title or a description", i)
		}
		// If enclosure present, ensure required attributes are valid
		if it.Enclosure != nil {
			if strings.TrimSpace(it.Enclosure.Url) == "" || strings.TrimSpace(it.Enclosure.Type) == "" || it.Enclosure.Length <= 0 {
				return fmt.Errorf("rss: item[%d] enclosure url/type/length required when enclosure present", i)
			}
		}
		// RSS 2.0 author should be an email address when present
		if it.Author != nil && strings.TrimSpace(it.Author.Email) == "" {
			return fmt.Errorf("rss: item[%d] author must be an email address", i)
		}
	}
	return nil
}
