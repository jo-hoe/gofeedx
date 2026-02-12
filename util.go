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
	data, err := xml.MarshalIndent(x, "", "  ")
	if err != nil {
		return "", err
	}
	// Trim the newline from the default header
	var buf bytes.Buffer
	buf.WriteString(xml.Header[:len(xml.Header)-1])
	buf.Write(data)
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