package ingest

import (
	"testing"
)

func TestChunkID_Deterministic(t *testing.T) {
	id1 := ChunkID("path/to/file.txt", "text", 0)
	id2 := ChunkID("path/to/file.txt", "text", 0)
	if id1 != id2 {
		t.Errorf("expected deterministic IDs, got %s != %s", id1, id2)
	}
}

func TestChunkID_Length(t *testing.T) {
	id := ChunkID("file.txt", "text", 0)
	// 20 bytes of SHA-256 → 40 hex chars
	if len(id) != 40 {
		t.Errorf("expected 40 hex chars, got %d: %s", len(id), id)
	}
}

func TestChunkID_DifferentPaths(t *testing.T) {
	id1 := ChunkID("a.txt", "text", 0)
	id2 := ChunkID("b.txt", "text", 0)
	if id1 == id2 {
		t.Error("expected different IDs for different paths")
	}
}

func TestChunkID_DifferentModalities(t *testing.T) {
	id1 := ChunkID("file.txt", "text", 0)
	id2 := ChunkID("file.txt", "image", 0)
	if id1 == id2 {
		t.Error("expected different IDs for different modalities")
	}
}

func TestChunkID_DifferentOffsets(t *testing.T) {
	id1 := ChunkID("file.txt", "text", 0)
	id2 := ChunkID("file.txt", "text", 1)
	if id1 == id2 {
		t.Error("expected different IDs for different offsets")
	}
}

func TestChunkID_EmptyPath(t *testing.T) {
	id := ChunkID("", "text", 0)
	if id == "" {
		t.Error("expected non-empty ID even for empty path")
	}
}
