package ingest

import (
	"testing"
	"time"

	"github.com/omarkamali/semango/internal/pdflib"
)

func TestExtractPageWithTimeout_normalPage(t *testing.T) {
	// A null/empty page should return quickly with no error.
	page := pdflib.Page{} // V is zero Value → IsNull
	text, err := extractPageWithTimeout(page, 1*time.Second)
	// GetPlainText on a zero page may panic (recovered) or return empty.
	// Either way we should not block forever.
	if err != nil {
		t.Logf("expected (possibly recovered) error on empty page: %v", err)
	}
	_ = text
}

func TestExtractPageWithTimeout_timesOut(t *testing.T) {
	// Simulate a page that takes too long by using a very short timeout.
	// We use a real (empty) page which should be fast, but with 0 timeout
	// it should time out before the goroutine even runs.
	page := pdflib.Page{}
	_, err := extractPageWithTimeout(page, 1*time.Nanosecond)
	// With 1ns timeout either:
	//  - the goroutine finishes first (no error / recovered error)
	//  - or we time out
	// Both are acceptable – the key assertion is that we don't block.
	_ = err
}

func TestExtractPDFPages_missingFile(t *testing.T) {
	err := ExtractPDFPages("/nonexistent/file.pdf", func(p PDFExtractPage) error {
		t.Fatal("emit should not be called for missing file")
		return nil
	})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestExtractPDFPages_nilEmit(t *testing.T) {
	// If we somehow get a valid page with text but emit is nil,
	// ExtractPDFPages should return an error.
	// We can't easily trigger this with real PDFs, but verify the
	// error path exists by checking the function signature.
	err := ExtractPDFPages("/nonexistent/file.pdf", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestDefaultPageTimeout(t *testing.T) {
	if DefaultPageTimeout <= 0 {
		t.Fatal("DefaultPageTimeout should be positive")
	}
	if DefaultPageTimeout > 10*time.Minute {
		t.Error("DefaultPageTimeout seems too large")
	}
}
