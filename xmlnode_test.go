package gofeedx

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func TestExtensionNode_MarshalXML_Order_Text_Children(t *testing.T) {
	n := ExtensionNode{
		Name: "root",
		Attrs: map[string]string{
			"b": "2",
			"a": "1",
		},
		Text: "X",
		Children: []ExtensionNode{
			{Name: "child", Text: "Y"},
		},
	}
	data, err := xml.Marshal(n)
	if err != nil {
		t.Fatalf("Marshal ExtensionNode: %v", err)
	}
	s := string(data)
	// Attributes should be sorted lexicographically by key
	if !strings.Contains(s, `<root a="1" b="2">`) {
		t.Errorf("expected sorted attributes, got %s", s)
	}
	if !strings.Contains(s, "<child>Y</child>") {
		t.Errorf("expected child node, got %s", s)
	}
	if !strings.Contains(s, ">X<") {
		t.Errorf("expected text content X, got %s", s)
	}
}

func TestEncodeElementIfSet_And_EncodeIntElementIfPositive(t *testing.T) {
	// encodeElementIfSet: empty string -> no element
	{
		var buf bytes.Buffer
		enc := xml.NewEncoder(&buf)
		if err := encodeElementIfSet(enc, "x", ""); err != nil {
			t.Fatalf("encodeElementIfSet(empty) error: %v", err)
		}
		_ = enc.Flush()
		if strings.Contains(buf.String(), "<x>") {
			t.Errorf("expected no <x> element for empty value, got %q", buf.String())
		}
	}
	// encodeElementIfSet: non-empty string -> element present
	{
		var buf bytes.Buffer
		enc := xml.NewEncoder(&buf)
		if err := encodeElementIfSet(enc, "x", "val"); err != nil {
			t.Fatalf("encodeElementIfSet(val) error: %v", err)
		}
		_ = enc.Flush()
		if !strings.Contains(buf.String(), "<x>val</x>") {
			t.Errorf("expected <x>val</x> element, got %q", buf.String())
		}
	}
	// encodeIntElementIfPositive: zero -> no element
	{
		var buf bytes.Buffer
		enc := xml.NewEncoder(&buf)
		if err := encodeIntElementIfPositive(enc, "n", 0); err != nil {
			t.Fatalf("encodeIntElementIfPositive(0) error: %v", err)
		}
		_ = enc.Flush()
		if strings.Contains(buf.String(), "<n>") {
			t.Errorf("expected no <n> element for non-positive value, got %q", buf.String())
		}
	}
	// encodeIntElementIfPositive: positive -> element present
	{
		var buf bytes.Buffer
		enc := xml.NewEncoder(&buf)
		if err := encodeIntElementIfPositive(enc, "n", 5); err != nil {
			t.Fatalf("encodeIntElementIfPositive(5) error: %v", err)
		}
		_ = enc.Flush()
		if !strings.Contains(buf.String(), "<n>5</n>") {
			t.Errorf("expected <n>5</n> element, got %q", buf.String())
		}
	}
}
