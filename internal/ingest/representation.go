package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// Representation holds a single piece of content extracted from a document.
// This aligns with spec.md §4 Data Model.
type Representation struct {
	ID       string            `json:"id"`        // Unique ID for this chunk/representation
	Path     string            `json:"path"`      // Original file path
	Modality string            `json:"modality"`  // e.g., "text", "image", "pdf_page"
	Text     string            `json:"text,omitempty"` // Text content, if applicable
	Vector   []float32         `json:"vector,omitempty"` // Vector embedding
	Preview  []byte            `json:"preview,omitempty"` // Thumbnail or preview data
	Meta     map[string]string `json:"meta,omitempty"`   // Additional metadata
	// Offset int64 `json:"offset,omitempty"` // Chunk offset, if applicable (for ID calculation)
}

// ChunkID calculates a unique ID for a piece of content (chunk).
// As per spec.md §4.2 Chunk ID Calculation.
func ChunkID(path, modality string, offset int64) string {
	// The spec uses `path + modality + strconv.FormatInt(offset,10)`.
	// For a whole file loaded as one chunk, offset can be 0.
	data := path + modality + strconv.FormatInt(offset, 10)
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:20]) // Use first 20 bytes of hash (40 hex chars)
}

// DocumentHeader as per spec.md (placeholder for now)
// type DocumentHeader struct {
// 	Path string
// 	// ... other fields
// }

// File as per spec.md (placeholder for now)
// type File struct {
// 	Path string
// 	// ... other fields
// } 