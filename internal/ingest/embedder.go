package ingest

import (
	"context"
	"fmt"
	"os"

	"github.com/omarkamali/semango/internal/config"
)

// Embedder defines the interface for embedding providers (OpenAI, local, etc.)
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// NewEmbedderFromConfig creates an embedder based on the provided configuration.
func NewEmbedderFromConfig(cfg config.EmbeddingConfig) (Embedder, error) {
	prov := cfg.Provider
	switch prov {
	case "openai", "": // default to openai
		apiKey := cfg.APIKey
		if apiKey == "" {
			envVar := cfg.APIKeyEnv
			if envVar == "" {
				envVar = "OPENAI_API_KEY"
			}
			apiKey = os.Getenv(envVar)
		}

		if apiKey == "" {
			return nil, fmt.Errorf("API key is required for openai embedder provider (set api_key, api_key_env, or OPENAI_API_KEY)")
		}

		baseURL := cfg.BaseURL
		if baseURL == "" {
			envVar := cfg.BaseURLEnv
			if envVar == "" {
				envVar = "OPENAI_BASE_URL"
			}
			baseURL = os.Getenv(envVar)
		}

		openCfg := OpenAIConfig{
			APIKey:     apiKey,
			BaseURL:    baseURL,
			Model:      cfg.Model,
			BatchSize:  cfg.BatchSize,
			Concurrent: cfg.Concurrent,
		}
		return NewOpenAIEmbedder(openCfg)
	case "local":
		if cfg.Model == "" {
			return nil, fmt.Errorf("embedding.model is required for local embedder provider")
		}
		localCfg := LocalEmbedderConfig{
			ModelPath:  cfg.Model,
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
