package ingest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTextLoader_Extensions(t *testing.T) {
	tl := NewTextLoader(100, 10)
	exts := tl.Extensions()
	want := map[string]bool{".txt": true, ".md": true, ".go": true}
	for _, e := range exts {
		if !want[e] {
			t.Errorf("unexpected extension: %s", e)
		}
	}
}

func TestTextLoader_SmallFileSingleChunk(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "small.txt")
	content := "hello world"
	os.WriteFile(f, []byte(content), 0o644)

	tl := NewTextLoader(1000, 0)
	reps, err := tl.Load(context.Background(), "small.txt", f)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(reps))
	}
	if reps[0].Text != content {
		t.Errorf("expected %q, got %q", content, reps[0].Text)
	}
	if reps[0].Path != "small.txt" {
		t.Errorf("expected path small.txt, got %s", reps[0].Path)
	}
	if reps[0].Modality != "text" {
		t.Errorf("expected modality text, got %s", reps[0].Modality)
	}
}

func TestTextLoader_Chunking(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "big.txt")
	// Build a long text with words separated by spaces
	words := make([]string, 50)
	for i := range words {
		words[i] = "word"
	}
	content := strings.Join(words, " ")
	os.WriteFile(f, []byte(content), 0o644)

	tl := NewTextLoader(20, 5)
	reps, err := tl.Load(context.Background(), "big.txt", f)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) < 2 {
		t.Fatalf("expected >1 chunks, got %d", len(reps))
	}

	// All chunks should have unique IDs
	ids := map[string]bool{}
	for _, r := range reps {
		if ids[r.ID] {
			t.Errorf("duplicate chunk ID: %s", r.ID)
		}
		ids[r.ID] = true
	}
}

func TestTextLoader_WhitespaceOnly(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "spaces.txt")
	os.WriteFile(f, []byte("      "), 0o644)

	tl := NewTextLoader(100, 0)
	reps, err := tl.Load(context.Background(), "spaces.txt", f)
	if err != nil {
		t.Fatal(err)
	}
	// Should produce a single chunk (whitespace is still content)
	if len(reps) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(reps))
	}
}

func TestTextLoader_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "empty.txt")
	os.WriteFile(f, []byte(""), 0o644)

	tl := NewTextLoader(100, 0)
	reps, err := tl.Load(context.Background(), "empty.txt", f)
	if err != nil {
		t.Fatal(err)
	}
	// Empty file → single empty chunk
	if len(reps) != 1 {
		t.Fatalf("expected 1 chunk for empty file, got %d", len(reps))
	}
}

func TestTextLoader_MissingFile(t *testing.T) {
	tl := NewTextLoader(100, 0)
	_, err := tl.Load(context.Background(), "missing.txt", "/nonexistent/missing.txt")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestTextLoader_DefaultChunkSize(t *testing.T) {
	tl := NewTextLoader(0, -1)
	if tl.chunkSize != 1000 {
		t.Errorf("expected default chunkSize=1000, got %d", tl.chunkSize)
	}
	if tl.overlap != 0 {
		t.Errorf("expected overlap clamped to 0, got %d", tl.overlap)
	}
}

func TestTextLoader_ChunkOverlapPreservesContent(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "overlap.txt")
	content := "alpha bravo charlie delta echo foxtrot golf hotel india juliet"
	os.WriteFile(f, []byte(content), 0o644)

	tl := NewTextLoader(20, 5)
	reps, err := tl.Load(context.Background(), "overlap.txt", f)
	if err != nil {
		t.Fatal(err)
	}

	// Reassembling (deduplicated) should cover the full content
	var combined strings.Builder
	seen := map[string]bool{}
	for _, r := range reps {
		for _, w := range strings.Fields(r.Text) {
			if !seen[w] {
				seen[w] = true
				if combined.Len() > 0 {
					combined.WriteByte(' ')
				}
				combined.WriteString(w)
			}
		}
	}

	for _, w := range strings.Fields(content) {
		if !seen[w] {
			t.Errorf("word %q not found in any chunk", w)
		}
	}
}

func TestCodeLoader_Extensions(t *testing.T) {
	cl := NewCodeLoader(false, 5*1024*1024)
	exts := cl.Extensions()
	if len(exts) == 0 {
		t.Error("expected non-empty extensions for CodeLoader")
	}
	found := false
	for _, e := range exts {
		if e == ".go" {
			found = true
		}
	}
	if !found {
		t.Error("expected .go in CodeLoader extensions")
	}
}

func TestCodeLoader_DetectLanguage(t *testing.T) {
	cl := NewCodeLoader(false, 5*1024*1024)
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"app.py", "python"},
		{"lib.rs", "rust"},
		{"index.js", "javascript"},
		{"main.ts", "typescript"},
		{"unknown.xyz", "unknown"},
	}
	for _, tt := range tests {
		got := cl.detectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestCodeLoader_SkipsLargeFiles(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "huge.go")
	data := make([]byte, 100)
	os.WriteFile(f, data, 0o644)

	cl := NewCodeLoader(false, 50) // maxFileSize = 50 bytes
	reps, err := cl.Load(context.Background(), "huge.go", f)
	if err != nil {
		t.Fatal(err)
	}
	if reps != nil {
		t.Error("expected nil reps for oversized file")
	}
}

func TestCodeLoader_LoadsSmallFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "main.go")
	os.WriteFile(f, []byte("package main\n"), 0o644)

	cl := NewCodeLoader(false, 5*1024*1024)
	reps, err := cl.Load(context.Background(), "main.go", f)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) != 1 {
		t.Fatalf("expected 1 rep, got %d", len(reps))
	}
	if reps[0].Meta["language"] != "go" {
		t.Errorf("expected language=go, got %s", reps[0].Meta["language"])
	}
}

func TestImageLoader_ReturnsNil(t *testing.T) {
	il := &ImageLoader{}
	reps, err := il.Load(context.Background(), "img.png", "/fake/img.png")
	if err != nil {
		t.Fatal(err)
	}
	if reps != nil {
		t.Error("expected nil from ImageLoader stub")
	}
}

func TestImageLoader_Extensions(t *testing.T) {
	il := &ImageLoader{}
	exts := il.Extensions()
	if len(exts) == 0 {
		t.Error("expected non-empty extensions")
	}
}

func TestSimpleChunker(t *testing.T) {
	c := &SimpleChunker{}
	chunks := c.Chunk("hello world")
	if len(chunks) != 1 || chunks[0] != "hello world" {
		t.Errorf("expected single chunk, got %v", chunks)
	}
}

func TestNoopEmbedder(t *testing.T) {
	e := &NoopEmbedder{}
	vecs, err := e.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	if e.Dimension() != 1 {
		t.Errorf("expected dimension 1, got %d", e.Dimension())
	}
}

func TestIsWordBoundary(t *testing.T) {
	boundaries := []byte{' ', '\t', '\n', '\r', '.', ',', ';', '!', '?'}
	for _, b := range boundaries {
		if !isWordBoundary(b) {
			t.Errorf("expected %q to be a word boundary", b)
		}
	}
	nonBoundaries := []byte{'a', 'Z', '0', '-', '_'}
	for _, b := range nonBoundaries {
		if isWordBoundary(b) {
			t.Errorf("expected %q to NOT be a word boundary", b)
		}
	}
}
