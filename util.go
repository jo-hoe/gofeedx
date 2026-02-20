package gofeedx

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
)

// XmlFeed is implemented by format wrappers to expose an XML-ready value.
type XmlFeed interface {
	FeedXml() interface{}
}

// ToXML marshals a feed wrapper to an XML string with the standard header (no trailing newline).
func ToXML(feed XmlFeed) (string, error) {
	x := feed.FeedXml()
	// Use xml.Encoder to ensure MarshalXML methods on writers are invoked
	var buf bytes.Buffer
	// Trim the newline from the default header
	buf.WriteString(xml.Header[:len(xml.Header)-1])
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(x); err != nil {
		return "", err
	}
	if err := enc.Flush(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// WriteXML writes a feed wrapper as XML to the provided writer, with header and indentation.
func WriteXML(feed XmlFeed, w io.Writer) error {
	x := feed.FeedXml()
	if _, err := w.Write([]byte(xml.Header[:len(xml.Header)-1])); err != nil {
		return err
	}
	e := xml.NewEncoder(w)
	e.Indent("", "  ")
	return e.Encode(x)
}

// WriteJSON writes a JSON value to the provided writer with indentation.
func WriteJSON(v any, w io.Writer) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
