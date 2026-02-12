package gofeedx

import (
	"encoding/json"
	"io"
	"strings"
	"time"
)

const jsonFeedVersion = "https://jsonfeed.org/version/1.1"

// JSONAuthor represents the author of the feed or of an individual item
type JSONAuthor struct {
	Name   string `json:"name,omitempty"`
	Url    string `json:"url,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

// JSONAttachment represents a related resource. (Kept for future expansion)
type JSONAttachment struct {
	Url      string        `json:"url,omitempty"`
	MIMEType string        `json:"mime_type,omitempty"`
	Title    string        `json:"title,omitempty"`
	Size     int32         `json:"size,omitempty"`
	Duration time.Duration `json:"-"`
}

// MarshalJSON implements the json.Marshaler interface.
func (a *JSONAttachment) MarshalJSON() ([]byte, error) {
	type EmbeddedJSONAttachment JSONAttachment
	type out struct {
		Duration float64 `json:"duration_in_seconds,omitempty"`
		*EmbeddedJSONAttachment
	}
	o := out{
		EmbeddedJSONAttachment: (*EmbeddedJSONAttachment)(a),
	}
	if a.Duration > 0 {
		o.Duration = a.Duration.Seconds()
	}
	return json.Marshal(o)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (a *JSONAttachment) UnmarshalJSON(data []byte) error {
	type EmbeddedJSONAttachment JSONAttachment
	var raw struct {
		Duration float64 `json:"duration_in_seconds,omitempty"`
		*EmbeddedJSONAttachment
	}
	raw.EmbeddedJSONAttachment = (*EmbeddedJSONAttachment)(a)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Duration > 0 {
		nsec := int64(raw.Duration * float64(time.Second))
		raw.EmbeddedJSONAttachment.Duration = time.Duration(nsec)
	}
	return nil
}

// JSONItem represents a single entry/post for the feed.
type JSONItem struct {
	Id            string           `json:"id"`
	Url           string           `json:"url,omitempty"`
	ExternalUrl   string           `json:"external_url,omitempty"`
	Title         string           `json:"title,omitempty"`
	ContentHTML   string           `json:"content_html,omitempty"`
	ContentText   string           `json:"content_text,omitempty"`
	Summary       string           `json:"summary,omitempty"`
	Image         string           `json:"image,omitempty"`
	BannerImage   string           `json:"banner_image,omitempty"`
	PublishedDate *time.Time       `json:"date_published,omitempty"`
	ModifiedDate  *time.Time       `json:"date_modified,omitempty"`
	Author        *JSONAuthor      `json:"author,omitempty"`  // deprecated in v1.1, kept for compatibility
	Authors       []*JSONAuthor    `json:"authors,omitempty"` // v1.1
	Tags          []string         `json:"tags,omitempty"`
	Attachments   []JSONAttachment `json:"attachments,omitempty"`
}

// JSONHub describes an endpoint that can be used to subscribe to real-time notifications.
type JSONHub struct {
	Type string `json:"type"`
	Url  string `json:"url"`
}

// JSONFeed represents a syndication feed in the JSON Feed Version 1.1 format.
type JSONFeed struct {
	Version     string        `json:"version"`
	Title       string        `json:"title"`
	Language    string        `json:"language,omitempty"`
	HomePageUrl string        `json:"home_page_url,omitempty"`
	FeedUrl     string        `json:"feed_url,omitempty"`
	Description string        `json:"description,omitempty"`
	UserComment string        `json:"user_comment,omitempty"`
	NextUrl     string        `json:"next_url,omitempty"`
	Icon        string        `json:"icon,omitempty"`
	Favicon     string        `json:"favicon,omitempty"`
	Author      *JSONAuthor   `json:"author,omitempty"`  // deprecated in v1.1, kept for compatibility
	Authors     []*JSONAuthor `json:"authors,omitempty"` // v1.1
	Expired     *bool         `json:"expired,omitempty"`
	Hubs        []*JSONHub    `json:"hubs,omitempty"`
	Items       []*JSONItem   `json:"items,omitempty"`
}

// JSON is used to convert a generic Feed to a JSONFeed.
type JSON struct {
	*Feed
}

// ToJSON encodes f into a JSON string. Returns an error if marshalling fails.
func (f *JSON) ToJSON() (string, error) {
	return f.JSONFeed().ToJSON()
}

// ToJSON encodes f into a JSON string. Returns an error if marshalling fails.
func (f *JSONFeed) ToJSON() (string, error) {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// JSONFeed creates a new JSONFeed with a generic Feed struct's data.
func (f *JSON) JSONFeed() *JSONFeed {
	feed := &JSONFeed{
		Version:     jsonFeedVersion,
		Title:       f.Title,
		Description: f.Description,
		Language:    f.Language,
	}

	if f.Link != nil {
		feed.HomePageUrl = f.Link.Href
	}
	if f.Author != nil {
		author := &JSONAuthor{
			Name: f.Author.Name,
		}
		feed.Author = author
		feed.Authors = []*JSONAuthor{author}
	}
	for _, e := range f.Items {
		feed.Items = append(feed.Items, newJSONItem(e))
	}
	return feed
}

func newJSONItem(i *Item) *JSONItem {
	item := &JSONItem{
		Id:          i.ID,
		Title:       i.Title,
		Summary:     i.Description,
		ContentHTML: i.Content, // Use HTML when Content present
	}

	if i.Link != nil {
		item.Url = i.Link.Href
	}
	if i.Source != nil {
		item.ExternalUrl = i.Source.Href
	}
	if i.Author != nil {
		author := &JSONAuthor{
			Name: i.Author.Name,
		}
		item.Author = author
		item.Authors = []*JSONAuthor{author}
	}
	if !i.Created.IsZero() {
		item.PublishedDate = &i.Created
	}
	if !i.Updated.IsZero() {
		item.ModifiedDate = &i.Updated
	}
	// If enclosure is an image, map to JSON Feed's "image"
	if i.Enclosure != nil && strings.HasPrefix(i.Enclosure.Type, "image/") {
		item.Image = i.Enclosure.Url
	}

	return item
}

// ToJSON creates a JSON Feed representation of this feed
func (f *Feed) ToJSON() (string, error) {
	j := &JSON{f}
	return j.ToJSON()
}

// WriteJSON writes a JSON representation of this feed to the writer.
func (f *Feed) WriteJSON(w io.Writer) error {
	j := &JSON{f}
	feed := j.JSONFeed()
	return WriteJSON(feed, w)
}