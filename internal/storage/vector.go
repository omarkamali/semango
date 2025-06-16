package storage

import "context"

// VectorResult defines the result structure for vector search.
type VectorResult struct {
	ID    string  `json:"id"`
	Score float32 `json:"score"`
}

// VectorIndex defines the interface for vector search (e.g., FAISS).
type VectorIndex interface {
	Upsert(ctx context.Context, id string, vector []float32) error
	Search(ctx context.Context, query []float32, topK int) ([]VectorResult, error)
	Dimension() int
	Close() error
}

// NoopVectorIndex is a stub implementation that does nothing.
type NoopVectorIndex struct{}

func (n *NoopVectorIndex) Upsert(ctx context.Context, id string, vector []float32) error { return nil }
func (n *NoopVectorIndex) Search(ctx context.Context, query []float32, topK int) ([]VectorResult, error) {
	return nil, nil
}
func (n *NoopVectorIndex) Dimension() int { return 0 }
func (n *NoopVectorIndex) Close() error   { return nil }
