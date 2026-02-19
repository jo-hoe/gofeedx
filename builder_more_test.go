package gofeedx

import (
	"strings"
	"testing"
)

func TestToAtom_NilFeedError(t *testing.T) {
	if _, err := ToAtom(nil); err == nil || !strings.Contains(err.Error(), "nil feed") {
		t.Fatalf("ToAtom expected nil feed error, got: %v", err)
	}
}

func TestToRSS_NilFeedError(t *testing.T) {
	if _, err := ToRSS(nil); err == nil || !strings.Contains(err.Error(), "nil feed") {
		t.Fatalf("ToRSS expected nil feed error, got: %v", err)
	}
}

func TestToPSP_NilFeedError(t *testing.T) {
	if _, err := ToPSP(nil); err == nil || !strings.Contains(err.Error(), "nil feed") {
		t.Fatalf("ToPSP expected nil feed error, got: %v", err)
	}
}

func TestToJSON_NilFeedError(t *testing.T) {
	if _, err := ToJSON(nil); err == nil || !strings.Contains(err.Error(), "nil feed") {
		t.Fatalf("ToJSON expected nil feed error, got: %v", err)
	}
}