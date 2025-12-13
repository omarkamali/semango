package storage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/blevesearch/go-faiss"
	"github.com/omarkamali/semango/internal/ingest"
)

func TestFaissIntegrationWithOpenAI(t *testing.T) {
	ctx := context.Background()

	// Start a local HTTP server to mock the OpenAI embeddings endpoint
	dim := 768
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only handle /embeddings path
		if r.URL.Path != "/embeddings" {
			http.NotFound(w, r)
			return
		}
		// Decode request into payload struct
		var payload struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		// Build dummy response
		type dataEntry struct {
			Index     int       `json:"index"`
			Object    string    `json:"object"`
			Embedding []float64 `json:"embedding"`
		}
		resp := struct {
			Object string      `json:"object"`
			Data   []dataEntry `json:"data"`
		}{
			Object: "list",
			Data:   make([]dataEntry, len(payload.Input)),
		}
		for i := range payload.Input {
			vec := make([]float64, dim)
			for j := range vec {
				vec[j] = 0.1
			}
			resp.Data[i] = dataEntry{Index: i, Object: "embedding", Embedding: vec}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer testServer.Close()

	// Configure the OpenAI embedder to use the test server
	embConfig := ingest.OpenAIConfig{
		APIKey:    "dummy",
		Model:     "text-embedding-nomic-embed-text-v1.5",
		BatchSize: 2,
		BaseURL:   testServer.URL,
	}
	emb, err := ingest.NewOpenAIEmbedder(embConfig)
	if err != nil {
		t.Fatalf("failed to create OpenAI embedder: %v", err)
	}

	texts := []string{"hello world", "hello world!"}
	vectors, err := emb.Embed(ctx, texts)
	if err != nil {
		t.Fatalf("failed to get embeddings: %v", err)
	}

	if len(vectors) != len(texts) {
		t.Fatalf("expected %d embeddings, got %d", len(texts), len(vectors))
	}

	// Create a temp path for the FAISS index
	idxPath := filepath.Join(os.TempDir(), "test_semango_faiss.index")
	defer os.Remove(idxPath)

	// Create a new FAISS index with inner product metric
	idx, err := NewFaissIndex(ctx, idxPath, emb.Dimension(), faiss.MetricInnerProduct)
	if err != nil {
		t.Fatalf("failed to create FAISS index: %v", err)
	}
	defer idx.Close(ctx)

	// Add vectors with IDs
	ids := []int64{1, 2}
	if err := idx.Add(ctx, vectors, ids); err != nil {
		t.Fatalf("failed to add vectors: %v", err)
	}

	// Search for the nearest neighbor of the first vector
	_, labels, err := idx.Search(ctx, vectors[0], 1)
	if err != nil {
		t.Fatalf("failed to search index: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("expected 1 result, got %d", len(labels))
	}
	if labels[0] != 1 {
		t.Errorf("expected nearest neighbor ID 1, got %d", labels[0])
	}
}
