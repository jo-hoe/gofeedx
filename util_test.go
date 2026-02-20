package gofeedx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

// errWriter helps simulate write failures.
type errWriter struct {
	failOnFirst bool
	writes      int
}

func (e *errWriter) Write(p []byte) (int, error) {
	e.writes++
	if e.failOnFirst && e.writes == 1 {
		return 0, errors.New("write error")
	}
	return 0, errors.New("write error")
}

func TestWriteXML_Success(t *testing.T) {
	feed := &Feed{
		Title:       "T",
		Link:        &Link{Href: "https://example.org/"},
		Description: "D",
	}
	feed.Items = []*Item{{Title: "I"}}
	xmlObj := (&Rss{feed}).RssFeed()

	var buf bytes.Buffer
	if err := WriteXML(xmlObj, &buf); err != nil {
		t.Fatalf("WriteXML() unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, xml.Header[:len(xml.Header)-1]) {
		t.Errorf("expected XML header without trailing newline")
	}
	if !strings.Contains(out, `<rss`) {
		t.Errorf("expected RSS root in output")
	}
}

func TestWriteXML_HeaderWriteError(t *testing.T) {
	feed := &Feed{
		Title:       "T",
		Link:        &Link{Href: "https://example.org/"},
		Description: "D",
		Items:       []*Item{{Title: "I"}},
	}
	xmlObj := (&Rss{feed}).RssFeed()
	ew := &errWriter{failOnFirst: true}
	if err := WriteXML(xmlObj, ew); err == nil {
		t.Fatalf("expected error from failing writer")
	}
}

func TestWriteJSON_SuccessAndError(t *testing.T) {
	// Success
	var buf bytes.Buffer
	if err := WriteJSON(map[string]any{"x": 1}, &buf); err != nil {
		t.Fatalf("WriteJSON() unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"x": 1`) {
		t.Errorf("expected serialized key")
	}
	// Error path using failing writer
	ew := &errWriter{failOnFirst: true}
	if err := WriteJSON(map[string]any{"x": 1}, ew); err == nil {
		t.Fatalf("expected WriteJSON to fail with writer error")
	}
}

func TestToXML_Success(t *testing.T) {
	feed := &Feed{
		Title:       "T",
		Link:        &Link{Href: "https://example.org/"},
		Description: "D",
		Items:       []*Item{{Title: "I"}},
	}
	xmlStr, err := ToXML(&Rss{feed})
	if err != nil {
		t.Fatalf("ToXML unexpected error: %v", err)
	}
	// Header should be present without trailing newline and RSS root present
	if !strings.HasPrefix(xmlStr, xml.Header[:len(xml.Header)-1]) {
		t.Errorf("expected XML header without trailing newline")
	}
	if !strings.Contains(xmlStr, "<rss") || !strings.Contains(xmlStr, "<channel>") {
		t.Errorf("expected RSS root and channel in output")
	}
}

// stepErrWriter simulates a writer that succeeds for the first write (XML header)
// and fails on subsequent writes (encoder body), to exercise the Encode error path.
type stepErrWriter struct {
	step int
}

func (w *stepErrWriter) Write(p []byte) (int, error) {
	w.step++
	if w.step == 1 {
		return len(p), nil
	}
	return 0, errors.New("write error")
}

func TestWriteXML_EncodeErrorOnBody(t *testing.T) {
	feed := &Feed{
		Title:       "T",
		Link:        &Link{Href: "https://example.org/"},
		Description: "D",
		Items:       []*Item{{Title: "I"}},
	}
	xmlObj := (&Rss{feed}).RssFeed()
	w := &stepErrWriter{}
	if err := WriteXML(xmlObj, w); err == nil {
		t.Fatalf("expected WriteXML to fail when body encoding write fails")
	}
}

// errMarshaler implements xml.Marshaler and returns an error to exercise ToXML error path.
type errMarshaler struct{}

func (e errMarshaler) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	return errors.New("marshal error")
}

// errFeedWrapper implements XmlFeed to return an errMarshaler, causing ToXML to fail.
type errFeedWrapper struct{}

func (w *errFeedWrapper) FeedXml() interface{} {
	return errMarshaler{}
}

func TestToXML_EncodeError(t *testing.T) {
	_, err := ToXML(&errFeedWrapper{})
	if err == nil || !strings.Contains(err.Error(), "marshal error") {
		t.Fatalf("expected ToXML encode error, got: %v", err)
	}
}
