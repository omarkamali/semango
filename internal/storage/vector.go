package storage

import "context"

// VectorIndex defines the interface for vector search (e.g., FAISS).
type VectorIndex interface {
	Upsert(id string, vector []float32) error
	Search(ctx context.Context, query []float32, topK int) ([]string, error)
	Dimension() int
	Close() error
}

// NoopVectorIndex is a stub implementation that does nothing.
type NoopVectorIndex struct{}

func (n *NoopVectorIndex) Upsert(id string, vector []float32) error { return nil }
func (n *NoopVectorIndex) Search(ctx context.Context, query []float32, topK int) ([]string, error) { return nil, nil }
func (n *NoopVectorIndex) Dimension() int { return 0 }
func (n *NoopVectorIndex) Close() error { return nil } 