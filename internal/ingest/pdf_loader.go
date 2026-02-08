package ingest

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/dslipak/pdf"
)

// PDFLoader handles PDF files.
type PDFLoader struct {
	chunkSize int
	overlap   int
	timeout   time.Duration
	heartbeat time.Duration
}

// NewPDFLoader returns a PDFLoader with chunk configuration.
type PDFLoaderOption func(*PDFLoader)

func WithPDFTimeout(d time.Duration) PDFLoaderOption {
	return func(pl *PDFLoader) {
		if d > 0 {
			pl.timeout = d
		}
	}
}

func WithPDFHeartbeatInterval(d time.Duration) PDFLoaderOption {
	return func(pl *PDFLoader) {
		if d > 0 {
			pl.heartbeat = d
		}
	}
}

func NewPDFLoader(chunkSize, overlap int, opts ...PDFLoaderOption) *PDFLoader {
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	if overlap < 0 {
		overlap = 0
	}
	pl := &PDFLoader{
		chunkSize: chunkSize,
		overlap:   overlap,
		timeout:   15 * time.Minute,
		heartbeat: 30 * time.Second,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(pl)
		}
	}
	return pl
}

func (pl *PDFLoader) Extensions() []string { return []string{".pdf"} }

func (pl *PDFLoader) Load(ctx context.Context, relPath string, absPath string) ([]Representation, error) {
	slog.Info("Loading PDF file", "relative_path", relPath, "absolute_path", absPath)

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	loadCtx := ctx
	var cancel context.CancelFunc
	if pl.timeout > 0 {
		loadCtx, cancel = context.WithTimeout(ctx, pl.timeout)
		defer cancel()
	}

	f, err := os.Open(absPath)
	if err != nil {
		slog.Error("Failed to open PDF file", "path", absPath, "error", err)
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// Ensure we don't hang indefinitely: on timeout/cancel, close the file to break IO.
	closeOnDone := make(chan struct{})
	go func() {
		select {
		case <-loadCtx.Done():
			_ = f.Close()
		case <-closeOnDone:
		}
	}()
	defer close(closeOnDone)

	var currentPage atomic.Int64
	start := time.Now()
	if pl.heartbeat > 0 {
		ticker := time.NewTicker(pl.heartbeat)
		defer ticker.Stop()
		go func() {
			for {
				select {
				case <-ticker.C:
					slog.Info("PDF extraction still running",
						"relative_path", relPath,
						"elapsed", time.Since(start).String(),
						"page", currentPage.Load(),
					)
				case <-loadCtx.Done():
					return
				case <-closeOnDone:
					return
				}
			}
		}()
	}

	// Guard against pdf.NewReader hanging: run it with the same timeout context.
	type readerResult struct {
		r   *pdf.Reader
		err error
	}
	readerCh := make(chan readerResult, 1)
	go func() {
		r, err := pdf.NewReader(f, fi.Size())
		readerCh <- readerResult{r: r, err: err}
	}()
	var r *pdf.Reader
	select {
	case res := <-readerCh:
		if res.err != nil {
			slog.Error("Failed to parse PDF file", "path", absPath, "error", res.err)
			return nil, res.err
		}
		r = res.r
	case <-loadCtx.Done():
		return nil, loadCtx.Err()
	}

	var allReps []Representation

	for i := 1; i <= r.NumPage(); i++ {
		currentPage.Store(int64(i))
		if err := loadCtx.Err(); err != nil {
			return nil, err
		}
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}

		// Guard against GetPlainText hanging.
		type textResult struct {
			text string
			err  error
		}
		textCh := make(chan textResult, 1)
		go func() {
			t, err := p.GetPlainText(nil)
			textCh <- textResult{text: t, err: err}
		}()
		var text string
		select {
		case res := <-textCh:
			if res.err != nil {
				slog.Warn("Failed to extract text from PDF page", "path", absPath, "page", i, "error", res.err)
				continue
			}
			text = res.text
		case <-loadCtx.Done():
			return nil, loadCtx.Err()
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
