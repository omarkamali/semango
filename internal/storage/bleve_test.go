package storage

import (
	"testing"
)

func TestBleveIndex_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	idxPath := tmpDir + "/test.bleve"
	idx, err := OpenOrCreateBleveIndex(idxPath)
	if err != nil {
		t.Fatalf("failed to open/create index: %v", err)
	}
	defer idx.Close()

	id := "doc1"
	text := "hello world this is a test document"
	meta := map[string]string{"foo": "bar"}
	if err := idx.IndexDocument(id, text, meta); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	hits, err := idx.SearchText("hello", 5)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(hits) == 0 || hits[0].ID != id {
		t.Errorf("expected hit for doc1, got %+v", hits)
	}

	doc, err := idx.GetDocument(id)
	if err != nil || doc == nil {
		t.Fatalf("failed to get document: %v", err)
	}
} 