package ingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalEmbedder_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  LocalEmbedderConfig
		wantErr bool
	}{
		{
			name: "valid config with local path",
			config: LocalEmbedderConfig{
				ModelPath: "/path/to/model",
				BatchSize: 32,
				MaxLength: 512,
			},
			wantErr: false,
		},
		{
			name: "valid config with supported ONNX model",
			config: LocalEmbedderConfig{
				ModelPath: "onnx-models/all-MiniLM-L6-v2-onnx",
				BatchSize: 32,
				MaxLength: 512,
			},
			wantErr: false,
		},
		{
			name: "empty model path",
			config: LocalEmbedderConfig{
				BatchSize: 32,
				MaxLength: 512,
			},
			wantErr: true,
		},
		{
			name: "invalid batch size",
			config: LocalEmbedderConfig{
				ModelPath: "/path/to/model",
				BatchSize: -1,
				MaxLength: 512,
			},
			wantErr: true,
		},
		{
			name: "unsupported HF model",
			config: LocalEmbedderConfig{
				ModelPath: "unsupported/model",
				BatchSize: 32,
				MaxLength: 512,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateModelConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateModelConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLocalEmbedder_GetSupportedModels(t *testing.T) {
	models := GetSupportedModels()
	if len(models) == 0 {
		t.Error("GetSupportedModels() returned empty list")
	}

	// Check that some expected models are present
	expectedModels := []string{
		"onnx-models/all-MiniLM-L6-v2-onnx",
		"onnx-models/all-mpnet-base-v2-onnx",
		"onnx-models/jina-embeddings-v2-small-en-onnx",
	}

	for _, expected := range expectedModels {
		found := false
		for _, model := range models {
			if model == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected model %s not found in supported models", expected)
		}
	}
}

func TestTokenizer_Tokenize(t *testing.T) {
	tokenizer := &Tokenizer{
		vocab: map[string]int{
			"hello": 1,
			"world": 2,
			"test":  3,
			"[UNK]": 0,
			"[PAD]": 4,
			"[CLS]": 5,
			"[SEP]": 6,
		},
		specialTokens: map[string]int{
			"[UNK]": 0,
			"[PAD]": 4,
			"[CLS]": 5,
			"[SEP]": 6,
		},
		unkToken: "[UNK]",
	}

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "simple text",
			text:     "hello world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "text with punctuation",
			text:     "hello, world!",
			expected: []string{"hello", ",", "world", "!"},
		},
		{
			name:     "empty text",
			text:     "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizer.tokenize(tt.text)
			if len(result) != len(tt.expected) {
				t.Errorf("tokenize() returned %d tokens, expected %d", len(result), len(tt.expected))
				return
			}
			for i, token := range result {
				if token != tt.expected[i] {
					t.Errorf("tokenize() token %d = %s, expected %s", i, token, tt.expected[i])
				}
			}
		})
	}
}

func TestTokenizer_ConvertTokensToIDs(t *testing.T) {
	tokenizer := &Tokenizer{
		vocab: map[string]int{
			"hello": 1,
			"world": 2,
			"test":  3,
			"[UNK]": 0,
			"[PAD]": 4,
			"[CLS]": 5,
			"[SEP]": 6,
		},
		specialTokens: map[string]int{
			"[UNK]": 0,
			"[PAD]": 4,
			"[CLS]": 5,
			"[SEP]": 6,
		},
		unkToken: "[UNK]",
	}

	tests := []struct {
		name     string
		tokens   []string
		expected []int
	}{
		{
			name:     "known tokens",
			tokens:   []string{"hello", "world"},
			expected: []int{1, 2},
		},
		{
			name:     "unknown tokens",
			tokens:   []string{"unknown", "tokens"},
			expected: []int{0, 0}, // UNK token ID
		},
		{
			name:     "mixed tokens",
			tokens:   []string{"hello", "unknown", "world"},
			expected: []int{1, 0, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizer.convertTokensToIDs(tt.tokens)
			if len(result) != len(tt.expected) {
				t.Errorf("convertTokensToIDs() returned %d IDs, expected %d", len(result), len(tt.expected))
				return
			}
			for i, id := range result {
				if id != tt.expected[i] {
					t.Errorf("convertTokensToIDs() ID %d = %d, expected %d", i, id, tt.expected[i])
				}
			}
		})
	}
}

func TestLocalEmbedder_MeanPooling(t *testing.T) {
	embedder := &LocalEmbedder{
		dimension: 3,
	}

	tokenEmbeddings := [][]float32{
		{1.0, 2.0, 3.0}, // CLS token
		{4.0, 5.0, 6.0}, // Token 1
		{7.0, 8.0, 9.0}, // Token 2
		{0.0, 0.0, 0.0}, // PAD token
	}
	attentionMask := []int64{1, 1, 1, 0} // PAD token masked out

	result := embedder.meanPooling(tokenEmbeddings, attentionMask)

	// Expected: (1+4+7)/3, (2+5+8)/3, (3+6+9)/3 = 4, 5, 6
	expected := []float32{4.0, 5.0, 6.0}

	if len(result) != len(expected) {
		t.Errorf("meanPooling() returned %d dimensions, expected %d", len(result), len(expected))
		return
	}

	for i, val := range result {
		if val != expected[i] {
			t.Errorf("meanPooling() dimension %d = %f, expected %f", i, val, expected[i])
		}
	}
}

func TestLocalEmbedder_MaxPooling(t *testing.T) {
	embedder := &LocalEmbedder{
		dimension: 3,
	}

	tokenEmbeddings := [][]float32{
		{1.0, 8.0, 3.0}, // CLS token
		{4.0, 2.0, 6.0}, // Token 1
		{7.0, 5.0, 9.0}, // Token 2
		{0.0, 0.0, 0.0}, // PAD token
	}
	attentionMask := []int64{1, 1, 1, 0} // PAD token masked out

	result := embedder.maxPooling(tokenEmbeddings, attentionMask)

	// Expected: max(1,4,7), max(8,2,5), max(3,6,9) = 7, 8, 9
	expected := []float32{7.0, 8.0, 9.0}

	if len(result) != len(expected) {
		t.Errorf("maxPooling() returned %d dimensions, expected %d", len(result), len(expected))
		return
	}

	for i, val := range result {
		if val != expected[i] {
			t.Errorf("maxPooling() dimension %d = %f, expected %f", i, val, expected[i])
		}
	}
}

func TestLocalEmbedder_NormalizeVector(t *testing.T) {
	embedder := &LocalEmbedder{}

	tests := []struct {
		name     string
		vector   []float32
		expected []float32
	}{
		{
			name:     "unit vector",
			vector:   []float32{1.0, 0.0, 0.0},
			expected: []float32{1.0, 0.0, 0.0},
		},
		{
			name:     "non-unit vector",
			vector:   []float32{3.0, 4.0, 0.0}, // magnitude = 5
			expected: []float32{0.6, 0.8, 0.0},
		},
		{
			name:     "zero vector",
			vector:   []float32{0.0, 0.0, 0.0},
			expected: []float32{0.0, 0.0, 0.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := embedder.normalizeVector(tt.vector)
			if len(result) != len(tt.expected) {
				t.Errorf("normalizeVector() returned %d dimensions, expected %d", len(result), len(tt.expected))
				return
			}

			for i, val := range result {
				if abs(val-tt.expected[i]) > 1e-6 {
					t.Errorf("normalizeVector() dimension %d = %f, expected %f", i, val, tt.expected[i])
				}
			}
		})
	}
}

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "absolute path",
			path:     "/path/to/model",
			expected: true,
		},
		{
			name:     "relative path with separator",
			path:     "./model/path",
			expected: true,
		},
		{
			name:     "hugging face model name",
			path:     "sentence-transformers/all-MiniLM-L6-v2",
			expected: false,
		},
		{
			name:     "simple model name",
			path:     "model-name",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalPath(tt.path)
			if result != tt.expected {
				t.Errorf("isLocalPath(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// Helper function for floating point comparison
func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// Integration test that creates a mock model directory
func TestLocalEmbedder_Integration(t *testing.T) {
	// Skip if ONNX runtime is not available
	t.Skip("Skipping integration test - requires ONNX runtime library")

	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "semango_test_model")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create mock model files
	if err := createMockModelFiles(tempDir); err != nil {
		t.Fatalf("Failed to create mock model files: %v", err)
	}

	// Test creating a local embedder with the mock model
	config := LocalEmbedderConfig{
		ModelPath: tempDir,
		BatchSize: 2,
		MaxLength: 128,
		CacheDir:  filepath.Join(tempDir, "cache"),
	}

	embedder, err := NewLocalEmbedder(config)
	if err != nil {
		t.Fatalf("Failed to create local embedder: %v", err)
	}
	defer embedder.Close()

	// Test embedding
	ctx := context.Background()
	texts := []string{"hello world", "test sentence"}

	embeddings, err := embedder.Embed(ctx, texts)
	if err != nil {
		t.Fatalf("Failed to embed texts: %v", err)
	}

	if len(embeddings) != len(texts) {
		t.Errorf("Expected %d embeddings, got %d", len(texts), len(embeddings))
	}

	// Check dimension
	expectedDim := embedder.Dimension()
	for i, emb := range embeddings {
		if len(emb) != expectedDim {
			t.Errorf("Embedding %d has dimension %d, expected %d", i, len(emb), expectedDim)
		}
	}
}

// createMockModelFiles creates the necessary files for a mock model
func createMockModelFiles(modelDir string) error {
	// Create vocab.txt
	vocabContent := `[PAD]
[UNK]
[CLS]
[SEP]
[MASK]
hello
world
test
sentence
`
	if err := os.WriteFile(filepath.Join(modelDir, "vocab.txt"), []byte(vocabContent), 0644); err != nil {
		return err
	}

	// Create pooling config directory and file
	poolingDir := filepath.Join(modelDir, "1_Pooling")
	if err := os.MkdirAll(poolingDir, 0755); err != nil {
		return err
	}

	poolingConfig := `{
  "word_embedding_dimension": 384,
  "pooling_mode_cls_token": false,
  "pooling_mode_mean_tokens": true,
  "pooling_mode_max_tokens": false,
  "pooling_mode_mean_sqrt_len_tokens": false,
  "include_prompt": true
}`
	if err := os.WriteFile(filepath.Join(poolingDir, "config.json"), []byte(poolingConfig), 0644); err != nil {
		return err
	}

	// Create a dummy ONNX model file (just an empty file for testing)
	if err := os.WriteFile(filepath.Join(modelDir, "model.onnx"), []byte("dummy"), 0644); err != nil {
		return err
	}

	return nil
}
