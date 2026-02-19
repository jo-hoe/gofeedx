package gofeedx

// RSS 2.0 encoder (with optional content:encoded for HTML content)
import (
	"encoding/xml"
	"errors"
	"fmt"
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
	*RssItemExtension
	Title       string      `xml:"title"` // required
	Link        string      `xml:"link"`  // required by spec, but often omitted by feeds
	Source      string      `xml:"source,omitempty"`
	Author      string      `xml:"author,omitempty"`
	Description string      `xml:"description"` // required by spec; we include if provided
	Content     *RssContent `xml:"content:encoded,omitempty"`
	Guid        *RssGuid
	PubDate     string `xml:"pubDate,omitempty"`
	Enclosure   *RssEnclosure
}

type RssItemExtension struct {
	XMLName  xml.Name        `xml:"item"`
	Category string          `xml:"category,omitempty"`
	Comments string          `xml:"comments,omitempty"`
	Extra    []ExtensionNode `xml:",any"` // custom nodes at item scope
}

type RssFeed struct {
	*RssFeedExtension
	Title          string     `xml:"title"`       // required
	Link           string     `xml:"link"`        // required
	Description    string     `xml:"description"` // required
	ManagingEditor string     `xml:"managingEditor,omitempty"`
	LastBuildDate  string     `xml:"lastBuildDate,omitempty"`
	PubDate        string     `xml:"pubDate,omitempty"`
	Items          []*RssItem `xml:"item"`
	Copyright      string     `xml:"copyright,omitempty"`
	Image          *RssImage  `xml:"image,omitempty"`
	Language       string     `xml:"language,omitempty"`
	Category       string     `xml:"category,omitempty"`
}

type RssFeedExtension struct {
	XMLName   xml.Name        `xml:"channel"`
	WebMaster string          `xml:"webMaster,omitempty"`
	Generator string          `xml:"generator,omitempty"`
	Docs      string          `xml:"docs,omitempty"`
	Cloud     string          `xml:"cloud,omitempty"`
	Ttl       int             `xml:"ttl,omitempty"`
	Rating    string          `xml:"rating,omitempty"`
	SkipHours string          `xml:"skipHours,omitempty"`
	SkipDays  string          `xml:"skipDays,omitempty"`
	Extra     []ExtensionNode `xml:",any"` // custom nodes at channel scope

}

// Rss is a wrapper to marshal a Feed as RSS 2.0.
type Rss struct {
	*Feed
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

	// Extract unified RSS builder markers from feed extensions
	imgW, imgH := 0, 0
	ttl := 0
	catOverride := ""
	var nonRSSExtras []ExtensionNode
	for _, n := range r.Extensions {
		switch n.Name {
		case "_rss:imageSize":
			if n.Attrs != nil {
				if s, ok := n.Attrs["width"]; ok {
					if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && v > 0 {
						imgW = v
					}
				}
				if s, ok := n.Attrs["height"]; ok {
					if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && v > 0 {
						imgH = v
					}
				}
			}
		case "_rss:ttl":
			if t := strings.TrimSpace(n.Text); t != "" {
				if v, err := strconv.Atoi(t); err == nil && v > 0 {
					ttl = v
				}
			}
		case "_rss:category":
			catOverride = strings.TrimSpace(n.Text)
		default:
			nonRSSExtras = append(nonRSSExtras, n)
		}
	}

	var image *RssImage
	if r.Image != nil {
		image = &RssImage{
			Url:    r.Image.Url,
			Title:  r.Image.Title,
			Link:   r.Image.Link,
			Width:  imgW,
			Height: imgH,
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
		RssFeedExtension: &RssFeedExtension{
			Ttl: ttl,
		},
	}

	// Category override or generic mapping
	if catOverride != "" {
		channel.Category = catOverride
	} else if len(r.Categories) > 0 && r.Categories[0] != nil && r.Categories[0].Text != "" {
		channel.Category = r.Categories[0].Text
	}

	// append items
	for _, it := range r.Items {
		channel.Items = append(channel.Items, newRssItem(it))
	}

	// append non-RSS builder extensions
	if len(nonRSSExtras) > 0 {
		channel.Extra = append(channel.Extra, nonRSSExtras...)
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
		if item.RssItemExtension == nil {
			item.RssItemExtension = &RssItemExtension{}
		}

		item.Extra = append(item.Extra, i.Extensions...)
	}
	return item
}

// ValidateRSS enforces basic RSS 2.0.1 requirements on the generic Feed.
func ValidateRSS(f *Feed) error {
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

/*
Unified typed builders for RSS channel fields.

Users can attach channel-level RSS fields via ExtOption without manually constructing nodes.
These helpers store internal markers that the RSS encoder consumes to set canonical struct fields.
This avoids stringly-typed public usage while keeping code colocated in rss.go.
*/

// RSSChannelFields holds channel-level RSS fields for unified builder.

// WithRSSChannel returns an ExtOption to set RSS channel fields.
// Internally stores markers consumed by the RSS encoder.

// WithRSSFeedExtension returns an ExtOption to append RSS channel-level nodes.

// WithRSSItemExtension returns an ExtOption to append RSS item-level nodes.
