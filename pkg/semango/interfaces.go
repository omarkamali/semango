package semango

import (
	"context"
)

// Embedder defines the interface for embedding providers.
// Implementations of this interface are responsible for converting
// text into vector embeddings.
type Embedder interface {
	// Embed takes a slice of texts and returns their vector embeddings.
	// It should handle batching and any provider-specific logic.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the dimensionality of the embeddings produced by this provider.
	Dimension() int
}

// Representation is a generic structure for content extracted by loaders.
// This is also defined in spec.md and might be used by Embedders or other components.
// For now, placing it here as it's a core type.
// TODO: Reconcile with `internal/ingest/representation.go` if it exists or move as appropriate.
type Representation struct {
	Modality string            `json:"modality"`
	Vector   []float32         `json:"vector,omitempty"`
	Text     string            `json:"text,omitempty"`
	Preview  []byte            `json:"preview,omitempty"` // e.g., image thumbnail
	Meta     map[string]string `json:"meta,omitempty"`
}

// DocumentHeader might also be a shared type.
// TODO: Define based on spec or move if already defined elsewhere.
type DocumentHeader struct {
	Path string `json:"path"`
	// Other fields like Source, LastModified etc. might go here.
}

// Loader defines the interface for file content loaders.
// Implementations are responsible for extracting representations
// from different file types.
type Loader interface {
	// Extensions returns a list of file extensions (e.g., ".txt", ".md")
	// or glob patterns that this loader can handle.
	Extensions() []string

	// Load reads a file at the given path and returns one or more
	// Representations. A single file might produce multiple representations
	// (e.g., a PDF with multiple pages, or a source code file chunked by functions).
	Load(ctx context.Context, path string) ([]Representation, error)
}
