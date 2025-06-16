package ingest

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Representation is defined in representation.go

// Loader defines the interface for loading different file types.
type Loader interface {
	Extensions() []string // List of file extensions this loader handles (e.g., [".txt", ".md"])
	Load(ctx context.Context, relPath string, absPath string) ([]Representation, error)
}

// TextLoader is a simple loader for plain text files.
type TextLoader struct {
	chunkSize int
	overlap   int
}

func (tl *TextLoader) Extensions() []string {
	return []string{".txt", ".md", ".go"} // Added .go
}

// NewTextLoader returns a TextLoader with chunk configuration.
func NewTextLoader(chunkSize, overlap int) *TextLoader {
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	if overlap < 0 {
		overlap = 0
	}
	return &TextLoader{chunkSize: chunkSize, overlap: overlap}
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

	// Chunking with word boundaries
	var reps []Representation
	size := tl.chunkSize
	ov := tl.overlap
	if size <= 0 || len(textContent) <= size {
		// Single chunk for small files
		chunkID := ChunkID(relPath, "text", 0)
		reps = append(reps, Representation{
			ID:       chunkID,
			Path:     relPath,
			Modality: "text",
			Text:     textContent,
			Meta: map[string]string{
				"source": "TextLoader",
				"offset": "0",
				"path":   relPath, // Explicitly store path in meta
			},
		})
		return reps, nil
	}

	start := 0
	offset := 0
	for start < len(textContent) {
		end := start + size
		if end > len(textContent) {
			end = len(textContent)
		}

		// Adjust end to word boundary (don't cut words in half)
		if end < len(textContent) {
			// Look backwards for a word boundary (space, newline, punctuation)
			for end > start && !isWordBoundary(textContent[end]) {
				end--
			}
			// If we couldn't find a word boundary, use the original end
			if end == start {
				end = start + size
			}
		}

		chunk := textContent[start:end]
		chunkID := ChunkID(relPath, "text", int64(offset))
		reps = append(reps, Representation{
			ID:       chunkID,
			Path:     relPath,
			Modality: "text",
			Text:     chunk,
			Meta: map[string]string{
				"source": "TextLoader",
				"offset": strconv.Itoa(start),
				"path":   relPath, // Explicitly store path in meta
			},
		})

		if end == len(textContent) {
			break
		}

		// Calculate next start position with overlap, respecting word boundaries
		nextStart := end - ov
		if nextStart <= start {
			nextStart = start + 1 // Ensure progress
		}

		// Adjust nextStart to word boundary
		for nextStart < len(textContent) && !isWordBoundary(textContent[nextStart]) {
			nextStart++
		}

		start = nextStart
		offset++
	}
	slog.Debug("Created", "chunks", len(reps), "relPath", relPath)
	return reps, nil
}

// isWordBoundary checks if a character is a word boundary
func isWordBoundary(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '.' || c == ',' || c == ';' || c == '!' || c == '?'
}

// CodeLoader loads and parses code files using Tree-sitter.
// It extracts semantic information and can optionally strip imports.
type CodeLoader struct {
	stripImports bool
	maxFileSize  int64 // Maximum file size in bytes (5MB as per spec)
}

// NewCodeLoader creates a new code loader with the given configuration.
func NewCodeLoader(stripImports bool, maxFileSize int64) *CodeLoader {
	if maxFileSize <= 0 {
		maxFileSize = 5 * 1024 * 1024 // 5MB default as per spec
	}

	return &CodeLoader{
		stripImports: stripImports,
		maxFileSize:  maxFileSize,
	}
}

func (cl *CodeLoader) Extensions() []string {
	return []string{
		".go", ".js", ".ts", ".py", ".jsx", ".tsx",
		".java", ".c", ".cpp", ".h", ".hpp", ".rs",
		".rb", ".php", ".cs", ".swift", ".kt", ".scala",
	}
}

func (cl *CodeLoader) Load(ctx context.Context, relPath string, absPath string) ([]Representation, error) {
	slog.Info("Loading code file with Tree-sitter", "relative_path", relPath, "absolute_path", absPath)

	// Check file size
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	if fileInfo.Size() > cl.maxFileSize {
		slog.Debug("Code file too large, skipping", "path", relPath, "size", fileInfo.Size(), "max_size", cl.maxFileSize)
		return nil, nil // Skip large files
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	// Determine language from file extension
	language := cl.detectLanguage(relPath)
	text := string(content)

	// For now, implement as a basic text loader with language detection
	// TODO: Implement full Tree-sitter parsing and import stripping
	chunkID := ChunkID(relPath, "text", 0)

	representation := Representation{
		ID:       chunkID,
		Path:     relPath,
		Modality: "text", // Code is treated as text modality
		Text:     text,
		Meta: map[string]string{
			"language":      language,
			"strip_imports": "false", // TODO: implement import stripping
			"source":        "CodeLoader",
			"file_size":     strconv.Itoa(len(content)),
			"path":          relPath, // Explicitly store path in meta
		},
	}

	slog.Debug("Successfully loaded code file", "relPath", relPath, "language", language, "size", len(content))
	return []Representation{representation}, nil
}

// detectLanguage determines the programming language from the file path.
func (cl *CodeLoader) detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	default:
		return "unknown"
	}
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
