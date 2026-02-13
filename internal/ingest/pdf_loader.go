package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
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

	closeOnDone := make(chan struct{})
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

	// PDF extraction is run in a subprocess so we can *reliably* abort on timeout
	// even if the PDF parser gets stuck in a CPU loop.
	allReps, err := pl.extractViaSubprocess(loadCtx, &currentPage, relPath, absPath)
	if err != nil {
		return nil, err
	}

	if len(allReps) == 0 {
		slog.Warn("No text extracted from PDF", "path", absPath)
		return nil, nil
	}

	slog.Debug("Created PDF chunks", "chunks", len(allReps), "relPath", relPath)
	return allReps, nil
}

func (pl *PDFLoader) extractViaSubprocess(ctx context.Context, currentPage *atomic.Int64, relPath, absPath string) ([]Representation, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	// Use base name to avoid leaking full paths in process listings where possible.
	cmd := exec.CommandContext(ctx, exe, "_pdf-extract", "--abs", absPath)
	cmd.Dir = filepath.Dir(absPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Read stderr concurrently for richer error messages.
	stderrDone := make(chan string, 1)
	go func() {
		b, _ := ioReadAllLimited(stderr, 64*1024)
		stderrDone <- string(b)
	}()

	dec := json.NewDecoder(stdout)
	var allReps []Representation
	for {
		var page PDFExtractPage
		err := dec.Decode(&page)
		if err != nil {
			if errors.Is(err, os.ErrClosed) || strings.Contains(err.Error(), "file already closed") {
				break
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				break
			}
			if errors.Is(err, io.EOF) {
				break
			}
			_ = cmd.Wait()
			stderrText := <-stderrDone
			if stderrText != "" {
				return nil, fmt.Errorf("pdf extract decode error: %w (stderr: %s)", err, stderrText)
			}
			return nil, fmt.Errorf("pdf extract decode error: %w", err)
		}

		if currentPage != nil {
			currentPage.Store(int64(page.Page))
		}
		if page.Text == "" {
			continue
		}
		// Clean extracted text: dehyphenate, remove replacement chars.
		cleaned := cleanExtractedText(page.Text)
		if cleaned == "" {
			continue
		}
		pageReps := pl.chunkText(cleaned, relPath, page.Page)
		allReps = append(allReps, pageReps...)
	}

	wErr := cmd.Wait()
	stderrText := <-stderrDone
	if wErr != nil {
		// If we timed out/canceled, return the context error so callers can count it as a failure.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if strings.TrimSpace(stderrText) != "" {
			return nil, fmt.Errorf("pdf extraction failed: %w (stderr: %s)", wErr, strings.TrimSpace(stderrText))
		}
		return nil, fmt.Errorf("pdf extraction failed: %w", wErr)
	}

	return allReps, nil
}

// ioReadAllLimited reads up to limit bytes.
func ioReadAllLimited(r io.Reader, limit int64) ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 64 * 1024
	}
	buf := &bytes.Buffer{}
	_, err := io.CopyN(buf, r, limit)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return buf.Bytes(), nil
		}
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}

func (pl *PDFLoader) chunkText(text string, relPath string, pageNum int) []Representation {
	// Use the shared rune-aware chunking. The base offset encodes the page
	// number so that chunk IDs stay unique across pages.
	reps := chunkTextGeneric(text, relPath, "PDFLoader", int64(pageNum*1000), pl.chunkSize, pl.overlap)

	// Enrich metadata with PDF-specific fields.
	for i := range reps {
		reps[i].Meta["page"] = strconv.Itoa(pageNum)
		reps[i].Meta["source"] = "PDFLoader"
		reps[i].Meta["ocr_failed"] = "false"
	}
	return reps
}
