package gofeedx

import (
	"testing"
	"time"
)

func TestAnyTimeFormat_FirstNonZero(t *testing.T) {
	z := time.Time{}
	t1 := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	got := anyTimeFormat(time.RFC3339, z, t1, t2)
	if got != t1.Format(time.RFC3339) {
		t.Errorf("anyTimeFormat got %q, want %q", got, t1.Format(time.RFC3339))
	}
	if anyTimeFormat(time.RFC3339, z, z) != "" {
		t.Errorf("anyTimeFormat expected empty for all zero inputs")
	}
}
