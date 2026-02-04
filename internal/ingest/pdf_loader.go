package ingest

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/dslipak/pdf"
)

// PDFLoader handles PDF files.
type PDFLoader struct {
	chunkSize int
	overlap   int
}

// NewPDFLoader returns a PDFLoader with chunk configuration.
func NewPDFLoader(chunkSize, overlap int) *PDFLoader {
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	if overlap < 0 {
		overlap = 0
	}
	return &PDFLoader{chunkSize: chunkSize, overlap: overlap}
}

func (pl *PDFLoader) Extensions() []string { return []string{".pdf"} }

func (pl *PDFLoader) Load(ctx context.Context, relPath string, absPath string) ([]Representation, error) {
	slog.Info("Loading PDF file", "relative_path", relPath, "absolute_path", absPath)

	r, err := pdf.Open(absPath)
	if err != nil {
		slog.Error("Failed to open PDF file", "path", absPath, "error", err)
		return nil, err
	}
	// rsc/pdf based readers don't have a Close() method on the reader itself
	// but they might leak file descriptors if not careful.
	// dslipak/pdf Open() opens the file.

	var allReps []Representation

	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			slog.Warn("Failed to extract text from PDF page", "path", absPath, "page", i, "error", err)
			continue
		}

		if len(text) == 0 {
			continue
		}

		// Chunk per page
		pageReps := pl.chunkText(text, relPath, i)
		allReps = append(allReps, pageReps...)
	}

	if len(allReps) == 0 {
		slog.Warn("No text extracted from PDF", "path", absPath)
		return nil, nil
	}

	slog.Debug("Created PDF chunks", "chunks", len(allReps), "relPath", relPath)
	return allReps, nil
}

func (pl *PDFLoader) chunkText(text string, relPath string, pageNum int) []Representation {
	var reps []Representation
	size := pl.chunkSize
	ov := pl.overlap

	if size <= 0 || len(text) <= size {
		chunkID := ChunkID(relPath, "pdf_page", int64(pageNum))
		reps = append(reps, Representation{
			ID:       chunkID,
			Path:     relPath,
			Modality: "text",
			Text:     text,
			Meta: map[string]string{
				"source":     "PDFLoader",
				"page":       strconv.Itoa(pageNum),
				"offset":     "0",
				"ocr_failed": "false",
				"path":       relPath,
			},
		})
		return reps
	}

	start := 0
	offset := 0
	for start < len(text) {
		end := start + size
		if end > len(text) {
			end = len(text)
		}

		if end < len(text) {
			for end > start && !isWordBoundary(text[end]) {
				end--
			}
			if end == start {
				end = start + size
			}
		}

		chunk := text[start:end]
		chunkID := ChunkID(relPath, "pdf_page", int64(pageNum*1000+offset))
		reps = append(reps, Representation{
			ID:       chunkID,
			Path:     relPath,
			Modality: "text",
			Text:     chunk,
			Meta: map[string]string{
				"source":     "PDFLoader",
				"page":       strconv.Itoa(pageNum),
				"offset":     strconv.Itoa(start),
				"ocr_failed": "false",
				"path":       relPath,
			},
		})

		if end == len(text) {
			break
		}

		nextStart := end - ov
		if nextStart <= start {
			nextStart = start + 1
		}
		for nextStart < len(text) && !isWordBoundary(text[nextStart]) {
			nextStart++
		}
		start = nextStart
		offset++
	}

	return reps
}
