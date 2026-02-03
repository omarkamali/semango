package ingest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestPDFLoader_Load(t *testing.T) {
	loader := NewPDFLoader(100, 10)

	tmpFile, err := os.CreateTemp("", "semango-test-*.pdf")
	if err != nil {
		t.Fatalf("failed to create temp pdf: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(buildTestPDF("Dummy PDF file")); err != nil {
		t.Fatalf("failed to write temp pdf: %v", err)
	}
	_ = tmpFile.Close()

	reps, err := loader.Load(context.Background(), "test.pdf", tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(reps) == 0 {
		t.Fatalf("no reps returned")
	}

	found := false
	for _, rep := range reps {
		if strings.Contains(rep.Text, "Dummy PDF file") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected text 'Dummy PDF file' not found in reps: %+v", reps)
	}
}

func buildTestPDF(text string) []byte {
	var buf bytes.Buffer
	write := func(s string) { _, _ = buf.WriteString(s) }

	write("%PDF-1.4\n")

	offsets := make([]int, 6)

	offsets[1] = buf.Len()
	write("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	offsets[2] = buf.Len()
	write("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	offsets[3] = buf.Len()
	write("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n")

	content := fmt.Sprintf("BT /F1 24 Tf 72 120 Td (%s) Tj ET", text)
	offsets[4] = buf.Len()
	write(fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content))

	offsets[5] = buf.Len()
	write("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	xrefOffset := buf.Len()
	write("xref\n0 6\n")
	write("0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		write(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	write("trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n")
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
	tmpFile.WriteString("not a pdf")
	tmpFile.Close()

	loader := NewPDFLoader(100, 10)
	_, err = loader.Load(context.Background(), "invalid.pdf", tmpFile.Name())
	if err == nil {
		t.Error("expected error for invalid pdf, got nil")
	}
}
