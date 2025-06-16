package ingest

import "context"

// Embedder defines the interface for embedding providers (OpenAI, Cohere, etc.)
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// NoopEmbedder is a stub implementation that returns zero vectors.
type NoopEmbedder struct{}

func (n *NoopEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = []float32{0}
	}
	return result, nil
}
func (n *NoopEmbedder) Dimension() int { return 1 } 