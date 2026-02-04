package ingest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ModelInfo contains metadata about a model.
type ModelInfo struct {
	ID          string `json:"id"`
	Alias       string `json:"alias,omitempty"`
	Size        string `json:"size"` // S, M, L
	VRAM        string `json:"vram"` // Estimated VRAM
	Description string `json:"description,omitempty"`
}

// DefaultModelCacheDir returns the default cache directory for downloaded ONNX models.
func DefaultModelCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".cache", "semango", "models"), nil
}

// SearchModelsOnline searches for compatible ONNX models on Hugging Face.
func SearchModelsOnline(query string) ([]ModelInfo, error) {
	baseURL := "https://huggingface.co/api/models"
	params := url.Values{}
	params.Add("author", "onnx-models")
	if query != "" {
		params.Add("search", query)
	}

	resp, err := http.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("failed to search models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to search models: status %d", resp.StatusCode)
	}

	var results []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	var models []ModelInfo
	for _, res := range results {
		info := GetModelMetadata(res.ID)
		models = append(models, info)
	}

	return models, nil
}

// GetModelMetadata returns metadata for a model ID, either from a known list or inferred.
func GetModelMetadata(id string) ModelInfo {
	// Standardize ID
	fullID := id
	if !strings.HasPrefix(fullID, "onnx-models/") {
		fullID = "onnx-models/" + id
	}

	// Known models
	metadata := map[string]ModelInfo{
		"onnx-models/all-MiniLM-L6-v2-onnx": {
			ID: "onnx-models/all-MiniLM-L6-v2-onnx", Alias: "all-MiniLM-L6-v2", Size: "S", VRAM: "128MB", Description: "Fast and lightweight, good for most use cases.",
		},
		"onnx-models/all-MiniLM-L12-v2-onnx": {
			ID: "onnx-models/all-MiniLM-L12-v2-onnx", Alias: "all-MiniLM-L12-v2", Size: "S", VRAM: "256MB", Description: "Slightly more accurate than L6, still very fast.",
		},
		"onnx-models/all-mpnet-base-v2-onnx": {
			ID: "onnx-models/all-mpnet-base-v2-onnx", Alias: "mpnet-base", Size: "M", VRAM: "512MB", Description: "High quality all-around embedding model.",
		},
		"onnx-models/bge-small-en-v1.5-onnx": {
			ID: "onnx-models/bge-small-en-v1.5-onnx", Alias: "bge-small", Size: "S", VRAM: "128MB", Description: "State-of-the-art small model from BAAI.",
		},
		"onnx-models/bge-base-en-v1.5-onnx": {
			ID: "onnx-models/bge-base-en-v1.5-onnx", Alias: "bge-base", Size: "M", VRAM: "512MB", Description: "State-of-the-art base model from BAAI.",
		},
		"onnx-models/bge-large-en-v1.5-onnx": {
			ID: "onnx-models/bge-large-en-v1.5-onnx", Alias: "bge-large", Size: "L", VRAM: "1.5GB", Description: "Very high quality, slower and larger footprint.",
		},
		"onnx-models/paraphrase-multilingual-MiniLM-L12-v2-onnx": {
			ID: "onnx-models/paraphrase-multilingual-MiniLM-L12-v2-onnx", Alias: "multilingual-mini", Size: "S", VRAM: "300MB", Description: "Good for multi-language support (50+ languages).",
		},
	}

	if info, ok := metadata[fullID]; ok {
		return info
	}

	// Inference for unknown models
	info := ModelInfo{ID: fullID}
	lowerID := strings.ToLower(fullID)

	if strings.Contains(lowerID, "small") || strings.Contains(lowerID, "mini") {
		info.Size = "S"
		info.VRAM = " ~256MB"
	} else if strings.Contains(lowerID, "large") || strings.Contains(lowerID, "xl") {
		info.Size = "L"
		info.VRAM = " ~2GB"
	} else {
		info.Size = "M"
		info.VRAM = " ~768MB"
	}

	// Try to extract an alias
	parts := strings.Split(fullID, "/")
	if len(parts) > 1 {
		info.Alias = strings.TrimSuffix(parts[1], "-onnx")
	}

	return info
}

// DownloadONNXModel downloads a model from the onnx-models organization to the cache directory.
func DownloadONNXModel(modelName, cacheDir string) (string, error) {
	if cacheDir == "" {
		return "", fmt.Errorf("cache directory is required")
	}
	le := &LocalEmbedder{}
	return le.downloadONNXModel(modelName, cacheDir)
}
