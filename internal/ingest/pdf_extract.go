package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/omarkamali/semango/internal/pdflib"
)

// PDFExtractPage is a single extracted page.
// This is intentionally small and JSON-friendly for streaming between processes.
type PDFExtractPage struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}

// DefaultPageTimeout is the maximum time allowed for extracting a single PDF page.
// If the parser hangs on a complex/malformed page, this prevents the process from
// blocking forever.
const DefaultPageTimeout = 2 * time.Minute

// ExtractPDFPages streams plain text per page from a PDF.
//
// This function does not take a context on purpose: the caller can run it inside a
// subprocess and enforce timeouts by killing the process.
func ExtractPDFPages(absPath string, emit func(PDFExtractPage) error) error {
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	r, err := pdflib.NewReader(f, fi.Size())
	if err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG: NewReader error: %v\n", err)
		return err
	}

	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := extractPageWithTimeout(p, DefaultPageTimeout)
		if err != nil {
			// Keep going; some pages may be malformed or slow.
			fmt.Fprintf(os.Stderr, "WARN: page %d: %v\n", i, err)
			continue
		}
		if text == "" {
			continue
		}
		if emit == nil {
			return fmt.Errorf("emit callback is nil")
		}
		if err := emit(PDFExtractPage{Page: i, Text: text}); err != nil {
			return err
		}
	}

	return nil
}

// extractPageWithTimeout runs GetPlainText in a goroutine with a per-page deadline.
// If the parser hangs (e.g. on a malformed content stream), we abandon that page
// rather than blocking the entire extraction.
func extractPageWithTimeout(p pdflib.Page, timeout time.Duration) (string, error) {
	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go func() {
		text, err := p.GetPlainText(nil)
		ch <- result{text, err}
	}()

	select {
	case res := <-ch:
		return res.text, res.err
	case <-ctx.Done():
		return "", fmt.Errorf("page extraction timed out after %v", timeout)
	}
}

// RunPDFExtract is the entry point for the internal _pdf-extract command.
func RunPDFExtract(absPath string) error {
	return ExtractPDFPages(absPath, func(page PDFExtractPage) error {
		data, err := json.Marshal(page)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	})
}
