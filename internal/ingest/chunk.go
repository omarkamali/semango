package ingest

// Chunker defines the interface for text chunking strategies.
type Chunker interface {
	Chunk(text string) []string
}

// SimpleChunker is a stub that returns the whole text as one chunk.
type SimpleChunker struct{}

func (s *SimpleChunker) Chunk(text string) []string {
	return []string{text}
} 