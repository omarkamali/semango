package ingest

import (
	"context"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// isRuneWordBoundary
// ---------------------------------------------------------------------------

func TestIsRuneWordBoundary(t *testing.T) {
	boundaries := []rune{' ', '\t', '\n', '\r', '.', ',', ';', '!', '?', ':', '-', '\u2014', '\u2013'}
	for _, r := range boundaries {
		if !isRuneWordBoundary(r) {
			t.Errorf("expected %q (U+%04X) to be a word boundary", r, r)
		}
	}

	nonBoundaries := []rune{'a', 'Z', '0', '\u00e9', '\u4e2d', '\U0001F96D'}
	for _, r := range nonBoundaries {
		if isRuneWordBoundary(r) {
			t.Errorf("expected %q (U+%04X) NOT to be a word boundary", r, r)
		}
	}
}

// ---------------------------------------------------------------------------
// dehyphenate
// ---------------------------------------------------------------------------

func TestDehyphenate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple hyphenated line break",
			input: "vo-\ncabulary",
			want:  "vocabulary",
		},
		{
			name:  "hyphen with trailing space before newline",
			input: "lan- \nguages",
			want:  "languages",
		},
		{
			name:  "hyphen with CRLF",
			input: "multi-\r\nlingual",
			want:  "multilingual",
		},
		{
			name:  "hyphen at end of text",
			input: "hello-",
			want:  "hello-",
		},
		{
			name:  "hyphen followed by uppercase (new sentence)",
			input: "end-\nThe next",
			want:  "end-\nThe next",
		},
		{
			name:  "no hyphens",
			input: "just plain text",
			want:  "just plain text",
		},
		{
			name:  "multiple hyphenated breaks",
			input: "pre-\ntrained multi-\nlingual",
			want:  "pretrained multilingual",
		},
		{
			name:  "hyphen mid-line preserved",
			input: "self-driving cars are great",
			want:  "self-driving cars are great",
		},
		{
			name:  "leading spaces on continuation",
			input: "vocab-\n  ulary",
			want:  "vocabulary",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dehyphenate(tc.input)
			if got != tc.want {
				t.Errorf("dehyphenate(%q)\n  got  %q\n  want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// sanitizeText
// ---------------------------------------------------------------------------

func TestSanitizeText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no replacement chars",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "replacement char removed",
			input: "hello \uFFFD world",
			want:  "hello  world",
		},
		{
			name:  "multiple replacement chars",
			input: "\uFFFD\uFFFD test \uFFFD",
			want:  " test ",
		},
		{
			name:  "preserves valid unicode",
			input: "caf\u00e9 r\u00e9sum\u00e9 \u65e5\u672c\u8a9e \U0001F96D",
			want:  "caf\u00e9 r\u00e9sum\u00e9 \u65e5\u672c\u8a9e \U0001F96D",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeText(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeText(%q)\n  got  %q\n  want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// cleanExtractedText (end-to-end)
// ---------------------------------------------------------------------------

func TestCleanExtractedText(t *testing.T) {
	input := "vo-\ncabulary \uFFFD model"
	want := "vocabulary  model"
	got := cleanExtractedText(input)
	if got != want {
		t.Errorf("cleanExtractedText(%q)\n  got  %q\n  want %q", input, got, want)
	}
}

// ---------------------------------------------------------------------------
// chunkTextGeneric
// ---------------------------------------------------------------------------

func TestChunkTextGeneric_singleChunk(t *testing.T) {
	reps := chunkTextGeneric("hello world", "test.txt", "TestSource", 0, 100, 10)
	if len(reps) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(reps))
	}
	if reps[0].Text != "hello world" {
		t.Errorf("unexpected text: %q", reps[0].Text)
	}
	if reps[0].Path != "test.txt" {
		t.Errorf("unexpected path: %q", reps[0].Path)
	}
}

func TestChunkTextGeneric_multipleChunks(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog and the cow"
	reps := chunkTextGeneric(text, "test.txt", "TestSource", 0, 20, 5)
	if len(reps) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(reps))
	}

	// Verify all text is covered
	var all strings.Builder
	for _, r := range reps {
		all.WriteString(r.Text)
		all.WriteString(" ")
	}
	for _, word := range strings.Fields(text) {
		if !strings.Contains(all.String(), word) {
			t.Errorf("word %q missing from chunks", word)
		}
	}
}

func TestChunkTextGeneric_UTF8Safety(t *testing.T) {
	text := "caf\u00e9 r\u00e9sum\u00e9 na\u00efve expos\u00e9 \u00fcber stra\u00dfe \u00dcniversit\u00e4t"
	reps := chunkTextGeneric(text, "test.txt", "TestSource", 0, 10, 3)

	for i, r := range reps {
		if !utf8.ValidString(r.Text) {
			t.Errorf("chunk %d is not valid UTF-8: %q", i, r.Text)
		}
	}
}

func TestChunkTextGeneric_CJKSafety(t *testing.T) {
	text := "\u65e5\u672c\u8a9e\u306e\u30c6\u30ad\u30b9\u30c8\u51e6\u7406\u306f\u6b63\u3057\u304f\u52d5\u4f5c\u3059\u308b\u3079\u304d\u3067\u3059"
	reps := chunkTextGeneric(text, "test.txt", "TestSource", 0, 5, 2)

	for i, r := range reps {
		if !utf8.ValidString(r.Text) {
			t.Errorf("chunk %d is not valid UTF-8: %q", i, r.Text)
		}
	}

	var all strings.Builder
	for _, r := range reps {
		all.WriteString(r.Text)
	}
	for _, r := range text {
		if !strings.ContainsRune(all.String(), r) {
			t.Errorf("rune %q (U+%04X) missing from chunks", string(r), r)
		}
	}
}

func TestChunkTextGeneric_EmojiSafety(t *testing.T) {
	text := "hello \U0001F96D world \U0001F30D test \U0001F680 done"
	reps := chunkTextGeneric(text, "test.txt", "TestSource", 0, 8, 2)

	for i, r := range reps {
		if !utf8.ValidString(r.Text) {
			t.Errorf("chunk %d is not valid UTF-8: %q", i, r.Text)
		}
	}
}

func TestChunkTextGeneric_EmptyText(t *testing.T) {
	reps := chunkTextGeneric("", "test.txt", "TestSource", 0, 100, 10)
	if len(reps) != 1 {
		t.Fatalf("expected 1 chunk for empty text, got %d", len(reps))
	}
	if reps[0].Text != "" {
		t.Errorf("expected empty text, got %q", reps[0].Text)
	}
}

func TestChunkTextGeneric_ProgressGuarantee(t *testing.T) {
	// 100 identical chars with no word boundaries — stress-tests the
	// forward-progress guarantee of the chunker.
	text := strings.Repeat("a", 100)
	reps := chunkTextGeneric(text, "test.txt", "TestSource", 0, 10, 3)

	if len(reps) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// Every chunk must be valid UTF-8 and non-empty (except possibly
	// the very last chunk which could be the tail).
	for i, r := range reps {
		if !utf8.ValidString(r.Text) {
			t.Errorf("chunk %d is not valid UTF-8", i)
		}
	}

	// With overlap, total rune count across all chunks should be >= 100.
	var totalLen int
	for _, r := range reps {
		totalLen += utf8.RuneCountInString(r.Text)
	}
	if totalLen < 100 {
		t.Errorf("chunks don't cover all content: total %d runes (with overlap) vs 100 original", totalLen)
	}
}

// ---------------------------------------------------------------------------
// Integration: TextLoader uses new chunking
// ---------------------------------------------------------------------------

func TestTextLoader_UTF8Chunking(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.txt"
	content := "caf\u00e9 r\u00e9sum\u00e9 na\u00efve expos\u00e9 \u00fcber stra\u00dfe \u00dcniversit\u00e4t \u65e5\u672c\u8a9e \U0001F96D emoji"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewTextLoader(15, 3)
	reps, err := loader.Load(context.Background(), "test.txt", path)
	if err != nil {
		t.Fatal(err)
	}

	for i, r := range reps {
		if !utf8.ValidString(r.Text) {
			t.Errorf("chunk %d is not valid UTF-8: %q", i, r.Text)
		}
	}
}

// ---------------------------------------------------------------------------
// Integration: PDFLoader.chunkText uses new chunking
// ---------------------------------------------------------------------------

func TestPDFLoader_ChunkText_UTF8(t *testing.T) {
	pl := NewPDFLoader(10, 3)
	text := "caf\u00e9 r\u00e9sum\u00e9 na\u00efve \u00fcber \u65e5\u672c\u8a9e"
	reps := pl.chunkText(text, "test.pdf", 1)

	for i, r := range reps {
		if !utf8.ValidString(r.Text) {
			t.Errorf("chunk %d is not valid UTF-8: %q", i, r.Text)
		}
		if r.Meta["source"] != "PDFLoader" {
			t.Errorf("chunk %d: expected source=PDFLoader, got %q", i, r.Meta["source"])
		}
		if r.Meta["page"] != "1" {
			t.Errorf("chunk %d: expected page=1, got %q", i, r.Meta["page"])
		}
	}
}

// ---------------------------------------------------------------------------
// Regression: old byte-based isWordBoundary still works for ASCII
// ---------------------------------------------------------------------------

func TestOldIsWordBoundary_StillWorks(t *testing.T) {
	if !isWordBoundary(' ') {
		t.Error("space should be boundary")
	}
	if isWordBoundary('a') {
		t.Error("'a' should not be boundary")
	}
}
