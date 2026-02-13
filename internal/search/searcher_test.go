package search

import (
	"testing"
)

func TestGetModality(t *testing.T) {
	tests := []struct {
		meta string
		path string
		want string
	}{
		{"", "file.go", "code"},
		{"", "file.py", "code"},
		{"", "file.png", "image"},
		{"", "file.mp3", "audio"},
		{"", "file.pdf", "pdf"},
		{"", "file.md", "text"},
		{"custom", "file.go", "custom"}, // meta takes priority
	}
	for _, tc := range tests {
		got := getModality(tc.meta, tc.path)
		if got != tc.want {
			t.Errorf("getModality(%q, %q) = %q, want %q", tc.meta, tc.path, got, tc.want)
		}
	}
}

func TestCreateHighlights(t *testing.T) {
	s := &Searcher{}

	t.Run("single match", func(t *testing.T) {
		hl := s.createHighlights("hello world", "hello")
		matches, ok := hl["text"]
		if !ok {
			t.Fatal("expected text highlights")
		}
		m := matches.([]map[string]int)
		if len(m) != 1 {
			t.Fatalf("expected 1 match, got %d", len(m))
		}
		if m[0]["start"] != 0 || m[0]["end"] != 5 {
			t.Errorf("unexpected match positions: %v", m[0])
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		hl := s.createHighlights("Hello World HELLO", "hello")
		m := hl["text"].([]map[string]int)
		if len(m) != 2 {
			t.Fatalf("expected 2 matches, got %d", len(m))
		}
	})

	t.Run("no match", func(t *testing.T) {
		hl := s.createHighlights("hello world", "xyz")
		if _, ok := hl["text"]; ok {
			t.Error("expected no text highlights for non-matching query")
		}
	})
}
