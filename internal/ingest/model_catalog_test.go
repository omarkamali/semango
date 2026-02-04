package ingest

import (
	"os"
	"strings"
	"testing"
)

func TestDefaultModelCacheDir(t *testing.T) {
	dir, err := DefaultModelCacheDir()
	if err != nil {
		t.Fatalf("DefaultModelCacheDir failed: %v", err)
	}
	if dir == "" {
		t.Error("Expected non-empty cache directory")
	}
	if !strings.Contains(dir, ".cache") || !strings.Contains(dir, "semango") {
		t.Errorf("Unexpected cache directory format: %s", dir)
	}
}

func TestGetModelMetadata(t *testing.T) {
	tests := []struct {
		name     string
		modelID  string
		wantSize string
		wantVRAM string
	}{
		{
			name:     "Known model: all-MiniLM-L6-v2-onnx",
			modelID:  "onnx-models/all-MiniLM-L6-v2-onnx",
			wantSize: "S",
			wantVRAM: "128MB",
		},
		{
			name:     "Known model: bge-large-en-v1.5-onnx",
			modelID:  "onnx-models/bge-large-en-v1.5-onnx",
			wantSize: "L",
			wantVRAM: "1.5GB",
		},
		{
			name:     "Inferred small model",
			modelID:  "onnx-models/some-mini-model",
			wantSize: "S",
			wantVRAM: " ~256MB",
		},
		{
			name:     "Inferred large model",
			modelID:  "onnx-models/extra-large-model",
			wantSize: "L",
			wantVRAM: " ~2GB",
		},
		{
			name:     "Inferred default (medium) model",
			modelID:  "onnx-models/some-generic-model",
			wantSize: "M",
			wantVRAM: " ~768MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetModelMetadata(tt.modelID)
			if got.Size != tt.wantSize {
				t.Errorf("GetModelMetadata().Size = %v, want %v", got.Size, tt.wantSize)
			}
			if got.VRAM != tt.wantVRAM {
				t.Errorf("GetModelMetadata().VRAM = %v, want %v", got.VRAM, tt.wantVRAM)
			}
			if got.ID != tt.modelID && !strings.HasSuffix(tt.modelID, got.ID) {
				t.Errorf("GetModelMetadata().ID = %v, want matching %v", got.ID, tt.modelID)
			}
		})
	}
}

func TestSearchModelsOnline_Basic(t *testing.T) {
	// Only run if network is available, otherwise skip
	if os.Getenv("CI") != "" || os.Getenv("NETWORK_TESTS") == "" {
		t.Skip("Skipping online search test; set NETWORK_TESTS=1 to run")
	}

	results, err := SearchModelsOnline("MiniLM")
	if err != nil {
		t.Fatalf("SearchModelsOnline failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one result for 'MiniLM'")
	}

	found := false
	for _, res := range results {
		if strings.Contains(strings.ToLower(res.ID), "minilm") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected search results to contain 'minilm' in their ID")
	}
}
