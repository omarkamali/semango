package ingest

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// isRuneWordBoundary reports whether r is a character on which we're happy to
// break a chunk.  It covers ASCII whitespace, common punctuation, and all
// Unicode space categories.
func isRuneWordBoundary(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	switch r {
	case '.', ',', ';', '!', '?', ':', '-', '—', '–':
		return true
	}
	return false
}

// dehyphenate removes soft-hyphens introduced by PDF line-wrapping.
// "vo-\ncabulary" → "vocabulary", but "self-driving\n" stays as "self-driving\n".
func dehyphenate(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])

		if r == '-' {
			// Look ahead: is the next non-whitespace on a new line?
			// Pattern: hyphen, optional spaces, newline, optional spaces, lowercase letter
			j := i + size
			// skip optional trailing spaces (before newline)
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			if j < len(s) && (s[j] == '\n' || s[j] == '\r') {
				// Skip newline (and possible \r\n)
				if s[j] == '\r' {
					j++
				}
				if j < len(s) && s[j] == '\n' {
					j++
				}
				// skip optional leading spaces on next line
				for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
					j++
				}
				// If the next char is a lowercase letter, it's a hyphenated word break
				if j < len(s) {
					nextR, _ := utf8.DecodeRuneInString(s[j:])
					if unicode.IsLower(nextR) {
						// Drop the hyphen and whitespace – rejoin the word
						i = j
						continue
					}
				}
			}
		}

		b.WriteRune(r)
		i += size
	}
	return b.String()
}

// sanitizeText removes Unicode replacement characters (U+FFFD) and collapses
// runs of whitespace produced by PDF extraction artifacts.
func sanitizeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == utf8.RuneError {
			continue // drop replacement characters
		}
		b.WriteRune(r)
	}
	return b.String()
}

// cleanExtractedText applies all post-processing to raw extracted text:
// dehyphenation, replacement-char removal, and whitespace normalisation.
func cleanExtractedText(s string) string {
	s = dehyphenate(s)
	s = sanitizeText(s)
	return s
}

// chunkTextGeneric splits text into overlapping chunks of approximately
// chunkSize runes, breaking at word boundaries.  It is UTF-8 safe —
// boundaries are always placed between rune boundaries.
//
// Each chunk is turned into a Representation using the provided metadata.
func chunkTextGeneric(text, relPath, source string, baseOffset int64, chunkSize, overlap int) []Representation {
	runes := []rune(text)
	totalRunes := len(runes)

	if chunkSize <= 0 || totalRunes <= chunkSize {
		chunkID := ChunkID(relPath, source, baseOffset)
		return []Representation{{
			ID:       chunkID,
			Path:     relPath,
			Modality: "text",
			Text:     text,
			Meta: map[string]string{
				"source": source,
				"offset": "0",
				"path":   relPath,
			},
		}}
	}

	var reps []Representation
	start := 0 // rune index
	seq := 0

	for start < totalRunes {
		end := start + chunkSize
		if end > totalRunes {
			end = totalRunes
		}

		// Walk backwards to a word boundary (rune-level)
		if end < totalRunes {
			origEnd := end
			for end > start && !isRuneWordBoundary(runes[end]) {
				end--
			}
			if end == start {
				end = origEnd // no boundary found — keep the full slice
			}
		}

		chunk := string(runes[start:end])
		chunkID := ChunkID(relPath, source, baseOffset+int64(seq))
		reps = append(reps, Representation{
			ID:       chunkID,
			Path:     relPath,
			Modality: "text",
			Text:     chunk,
			Meta: map[string]string{
				"source": source,
				"offset": strconv.Itoa(start),
				"path":   relPath,
			},
		})

		if end >= totalRunes {
			break
		}

		// Next start: back up by overlap amount
		nextStart := end - overlap
		if nextStart <= start {
			nextStart = start + 1
		}

		// Try to align to a word boundary: walk forward, but only within
		// a limited range so we don't skip large amounts of content (e.g.
		// CJK text with no spaces or long strings with no boundaries).
		maxScan := overlap // don't scan more than overlap runes forward
		scan := 0
		aligned := nextStart
		for aligned < totalRunes && scan < maxScan && !isRuneWordBoundary(runes[aligned]) {
			aligned++
			scan++
		}
		if aligned < totalRunes && isRuneWordBoundary(runes[aligned]) {
			// Found a boundary — skip the boundary char so chunk starts
			// with the next word
			nextStart = aligned + 1
		}
		// Otherwise keep nextStart as is (mid-word is okay to avoid gaps)

		if nextStart <= start {
			nextStart = end // ensure forward progress
		}

		start = nextStart
		seq++
	}
	return reps
}
