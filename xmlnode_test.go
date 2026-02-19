package gofeedx

import (
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