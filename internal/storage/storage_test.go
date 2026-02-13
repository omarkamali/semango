package storage

import (
	"context"
	"testing"

	"github.com/omarkamali/semango/internal/ingest"
)

func TestInMemoryStore_AddAndGet(t *testing.T) {
	store := NewInMemoryStore()

	rep := ingest.Representation{ID: "r1", Text: "hello"}
	if err := store.Add(rep); err != nil {
		t.Fatal(err)
	}

	got, ok := store.Get("r1")
	if !ok {
		t.Fatal("expected to find r1")
	}
	if got.Text != "hello" {
		t.Errorf("expected text hello, got %s", got.Text)
	}
}

func TestInMemoryStore_Count(t *testing.T) {
	store := NewInMemoryStore()
	if store.Count() != 0 {
		t.Errorf("expected 0, got %d", store.Count())
	}

	store.Add(ingest.Representation{ID: "r1"})
	store.Add(ingest.Representation{ID: "r2"})
	if store.Count() != 2 {
		t.Errorf("expected 2, got %d", store.Count())
	}
}

func TestInMemoryStore_OverwriteSameID(t *testing.T) {
	store := NewInMemoryStore()
	store.Add(ingest.Representation{ID: "r1", Text: "v1"})
	store.Add(ingest.Representation{ID: "r1", Text: "v2"})

	if store.Count() != 1 {
		t.Errorf("expected count=1 after overwrite, got %d", store.Count())
	}
	got, _ := store.Get("r1")
	if got.Text != "v2" {
		t.Errorf("expected overwritten text v2, got %s", got.Text)
	}
}

func TestInMemoryStore_GetAll_Order(t *testing.T) {
	store := NewInMemoryStore()
	store.Add(ingest.Representation{ID: "b"})
	store.Add(ingest.Representation{ID: "a"})
	store.Add(ingest.Representation{ID: "c"})

	all := store.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	// Insertion order preserved
	if all[0].ID != "b" || all[1].ID != "a" || all[2].ID != "c" {
		t.Errorf("unexpected order: %v", []string{all[0].ID, all[1].ID, all[2].ID})
	}
}

func TestInMemoryStore_GetMissing(t *testing.T) {
	store := NewInMemoryStore()
	_, ok := store.Get("missing")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestNoopVectorIndex(t *testing.T) {
	idx := &NoopVectorIndex{}
	if idx.Dimension() != 0 {
		t.Errorf("expected 0 dimension")
	}
	if err := idx.Upsert(context.Background(), "id", nil); err != nil {
		t.Fatal(err)
	}
	results, err := idx.Search(context.Background(), nil, 5)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Error("expected nil results from noop")
	}
	if err := idx.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBleveIndex_DeleteAndDocCount(t *testing.T) {
	tmpDir := t.TempDir()
	idxPath := tmpDir + "/test.bleve"
	idx, err := OpenOrCreateBleveIndex(idxPath)
	if err != nil {
		t.Fatalf("failed to open/create index: %v", err)
	}
	defer idx.Close()

	// Index two documents
	idx.IndexDocument("d1", "hello world", "a.txt", nil)
	idx.IndexDocument("d2", "goodbye world", "b.txt", nil)

	cnt, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if cnt != 2 {
		t.Errorf("expected 2 docs, got %d", cnt)
	}

	// Delete one
	if err := idx.Delete("d1"); err != nil {
		t.Fatal(err)
	}
	cnt, _ = idx.DocCount()
	if cnt != 1 {
		t.Errorf("expected 1 doc after delete, got %d", cnt)
	}
}

func TestBleveIndex_GetAllDocs(t *testing.T) {
	tmpDir := t.TempDir()
	idxPath := tmpDir + "/test.bleve"
	idx, err := OpenOrCreateBleveIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	idx.IndexDocument("d1", "text1", "a.txt", nil)
	idx.IndexDocument("d2", "text2", "b.txt", nil)

	all, err := idx.GetAllDocs()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(all))
	}
	if all["d1"] != "a.txt" || all["d2"] != "b.txt" {
		t.Errorf("unexpected doc mapping: %v", all)
	}
}

func TestBleveIndex_Reopen(t *testing.T) {
	tmpDir := t.TempDir()
	idxPath := tmpDir + "/test.bleve"

	// Create and index
	idx, _ := OpenOrCreateBleveIndex(idxPath)
	idx.IndexDocument("d1", "persist test", "file.txt", nil)
	idx.Close()

	// Reopen
	idx2, err := OpenOrCreateBleveIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer idx2.Close()

	cnt, _ := idx2.DocCount()
	if cnt != 1 {
		t.Errorf("expected 1 doc after reopen, got %d", cnt)
	}
}

func TestBleveIndex_SearchNoResults(t *testing.T) {
	tmpDir := t.TempDir()
	idx, _ := OpenOrCreateBleveIndex(tmpDir + "/test.bleve")
	defer idx.Close()

	hits, err := idx.SearchText("nonexistent", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits, got %d", len(hits))
	}
}

func TestBleveIndex_GetDocument_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	idx, _ := OpenOrCreateBleveIndex(tmpDir + "/test.bleve")
	defer idx.Close()

	doc, err := idx.GetDocument("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if doc != nil {
		t.Error("expected nil doc for nonexistent ID")
	}
}
