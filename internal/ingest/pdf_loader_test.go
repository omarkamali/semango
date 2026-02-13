package ingest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// If the binary is run with the internal _pdf-extract command, handle it.
	// This happens during tests because PDFLoader calls os.Executable() to run the extraction in a subprocess.
	for i, arg := range os.Args {
		if arg == "_pdf-extract" {
			var absPath string
			for j := i + 1; j < len(os.Args); j++ {
				if os.Args[j] == "--abs" && j+1 < len(os.Args) {
					absPath = os.Args[j+1]
					break
				}
			}
			if absPath != "" {
				if err := RunPDFExtract(absPath); err != nil {
					fmt.Fprintf(os.Stderr, "internal _pdf-extract failed: %v\n", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
	}
	os.Exit(m.Run())
}

func TestPDFLoader_Load(t *testing.T) {
	loader := NewPDFLoader(100, 10)

	tmpFile, err := os.CreateTemp("", "semango-test-*.pdf")
	if err != nil {
		t.Fatalf("failed to create temp pdf: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Test with a 2-page PDF
	pdfData := buildTestPDF([]string{"Page one content", "Page two content"})
	if _, err := tmpFile.Write(pdfData); err != nil {
		t.Fatalf("failed to write temp pdf: %v", err)
	}
	_ = tmpFile.Close()

	reps, err := loader.Load(context.Background(), "test.pdf", tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(reps) < 2 {
		t.Fatalf("expected at least 2 reps (one per page), got %d", len(reps))
	}

	// Verify metadata and content
	foundPage1 := false
	foundPage2 := false
	for _, rep := range reps {
		if strings.Contains(rep.Text, "Page one content") {
			foundPage1 = true
			if rep.Meta["page"] != "1" {
				t.Errorf("expected page meta '1' for page 1 content, got %s", rep.Meta["page"])
			}
		}
		if strings.Contains(rep.Text, "Page two content") {
			foundPage2 = true
			if rep.Meta["page"] != "2" {
				t.Errorf("expected page meta '2' for page 2 content, got %s", rep.Meta["page"])
			}
		}
		if rep.Meta["ocr_failed"] != "false" {
			t.Errorf("expected ocr_failed='false', got %s", rep.Meta["ocr_failed"])
		}
	}

	if !foundPage1 {
		t.Error("Page one content not found")
	}
	if !foundPage2 {
		t.Error("Page two content not found")
	}
}

// buildTestPDF creates a minimal multi-page PDF for testing.
func buildTestPDF(pages []string) []byte {
	var buf bytes.Buffer
	write := func(s string) { _, _ = buf.WriteString(s) }

	write("%PDF-1.4\n")

	// Objects:
	// 1: Catalog
	// 2: Pages (Parent)
	// 3..3+(n-1): Page objects
	// 3+n..3+2n-1: Content streams
	// 3+2n: Font
	n := len(pages)
	offsets := make([]int, 3+2*n+1)

	// Catalog
	offsets[1] = buf.Len()
	write("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Pages
	kids := ""
	for i := 0; i < n; i++ {
		kids += fmt.Sprintf("%d 0 R ", 3+i)
	}
	offsets[2] = buf.Len()
	write(fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", kids, n))

	// Page objects
	for i := 0; i < n; i++ {
		offsets[3+i] = buf.Len()
		write(fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R >> >> >>\nendobj\n", 3+i, 3+n+i, 3+2*n))
	}

	// Content streams
	for i := 0; i < n; i++ {
		content := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", pages[i])
		offsets[3+n+i] = buf.Len()
		write(fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", 3+n+i, len(content), content))
	}

	// Font
	offsets[3+2*n] = buf.Len()
	write(fmt.Sprintf("%d 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n", 3+2*n))

	xrefOffset := buf.Len()
	numObjs := 3 + 2*n
	write(fmt.Sprintf("xref\n0 %d\n", numObjs+1))
	write("0000000000 65535 f \n")
	for i := 1; i <= numObjs; i++ {
		write(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	write(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n", numObjs+1))
	write(fmt.Sprintf("%d\n%%%%EOF\n", xrefOffset))

	return buf.Bytes()
}

func TestPDFLoader_Load_FileNotFound(t *testing.T) {
	loader := NewPDFLoader(100, 10)
	_, err := loader.Load(context.Background(), "missing.pdf", "/tmp/non-existent-file-123.pdf")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestPDFLoader_Load_InvalidFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "*.pdf")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	_, _ = tmpFile.WriteString("not a pdf")
	_ = tmpFile.Close()

	loader := NewPDFLoader(100, 10)
	_, err = loader.Load(context.Background(), "invalid.pdf", tmpFile.Name())
	if err == nil {
		t.Error("expected error for invalid pdf, got nil")
	}
}
