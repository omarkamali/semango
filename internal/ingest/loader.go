package ingest

import (
	"context"
	"log/slog"
	"os"
)

// Representation is defined in representation.go

// Loader defines the interface for loading different file types.
type Loader interface {
	Extensions() []string // List of file extensions this loader handles (e.g., [".txt", ".md"])
	Load(ctx context.Context, relPath string, absPath string) ([]Representation, error)
}

// TextLoader is a simple loader for plain text files.
type TextLoader struct{}

func (tl *TextLoader) Extensions() []string {
	return []string{".txt", ".md", ".go"} // Added .go
}

// Load now takes relPath and absPath.
// relPath is used for ChunkID and stored in Representation.Path.
// absPath is used to read the file content.
func (tl *TextLoader) Load(ctx context.Context, relPath string, absPath string) ([]Representation, error) {
	slog.Info("Loading text file", "relative_path", relPath, "absolute_path", absPath)

	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		slog.Error("Failed to read file for TextLoader", "path", absPath, "error", err)
		return nil, err
	}
	textContent := string(contentBytes)

	// Use relPath for ChunkID for consistency and portability
	chunkID := ChunkID(relPath, "text", 0)

	reps := []Representation{
		{
			ID:       chunkID,
			Path:     relPath,    // Store the relative path
			Modality: "text",
			Text:     textContent,
			Meta:     map[string]string{"source": "TextLoader"},
		},
	}
	slog.Debug("Created representation for text file", "relPath", relPath, "id", chunkID, "text_length", len(textContent))
	return reps, nil
}

// CodeLoader is a stub for code files (to be implemented with Tree-sitter)
type CodeLoader struct{}
func (cl *CodeLoader) Extensions() []string { return []string{".go", ".py", ".js", ".ts"} }
func (cl *CodeLoader) Load(ctx context.Context, relPath string, absPath string) ([]Representation, error) {
	// TODO: Implement code parsing and chunking
	return nil, nil
}

// PDFLoader is a stub for PDF files
type PDFLoader struct{}
func (pl *PDFLoader) Extensions() []string { return []string{".pdf"} }
func (pl *PDFLoader) Load(ctx context.Context, relPath string, absPath string) ([]Representation, error) {
	// TODO: Implement PDF text extraction and OCR
	return nil, nil
}

// ImageLoader is a stub for image files
type ImageLoader struct{}
func (il *ImageLoader) Extensions() []string { return []string{".png", ".jpg", ".jpeg"} }
func (il *ImageLoader) Load(ctx context.Context, relPath string, absPath string) ([]Representation, error) {
	// TODO: Implement image embedding and alt-text extraction
	return nil, nil
}

// TODO: Add other loaders (Code, PDF, Image) as per spec.md 