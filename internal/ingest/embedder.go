package ingest

import (
	"context"
	"fmt"
	"os"

	"github.com/omarkamali/semango/internal/config"
)

// Embedder defines the interface for embedding providers (OpenAI, Cohere, etc.)
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// NewEmbedderFromConfig creates an embedder based on the provided configuration.
func NewEmbedderFromConfig(cfg config.EmbeddingConfig) (Embedder, error) {
	prov := cfg.Provider
	switch prov {
	case "openai", "": // default to openai
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required but not found in OPENAI_API_KEY environment variable")
		}
		openCfg := OpenAIConfig{
			APIKey:     apiKey,
			Model:      cfg.Model,
			BatchSize:  cfg.BatchSize,
			Concurrent: cfg.Concurrent,
		}
		return NewOpenAIEmbedder(openCfg)
	case "local":
		if cfg.LocalModelPath == "" {
			return nil, fmt.Errorf("local model path is required for local embedder provider")
		}
		localCfg := LocalEmbedderConfig{
			ModelPath:  cfg.LocalModelPath,
			CacheDir:   cfg.ModelCacheDir,
			BatchSize:  cfg.BatchSize,
			MaxLength:  512, // Default max length
			OutputName: cfg.OnnxOutputName,
		}
		// Validate configuration
		if err := ValidateModelConfig(localCfg); err != nil {
			return nil, fmt.Errorf("invalid local embedder configuration: %w", err)
		}
		return NewLocalEmbedder(localCfg)
	default:
		return nil, fmt.Errorf("unsupported embedder provider: %s. Supported providers: openai, local", prov)
	}
}

// NoopEmbedder is a stub implementation that returns zero vectors.
type NoopEmbedder struct{}

func (n *NoopEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = []float32{0}
	}
	return result, nil
}
func (n *NoopEmbedder) Dimension() int { return 1 }
