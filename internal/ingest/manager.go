package ingest

import "context"

// RepresentationManager coordinates loading, chunking, and indexing.
type RepresentationManager struct{}

func NewRepresentationManager() *RepresentationManager {
	return &RepresentationManager{}
}

func (m *RepresentationManager) ProcessFile(ctx context.Context, relPath, absPath string) error {
	// TODO: Implement unified pipeline: choose loader, chunk, embed, index
	return nil
} 