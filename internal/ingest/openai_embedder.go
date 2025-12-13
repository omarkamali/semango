package ingest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
	"golang.org/x/time/rate"

	"github.com/omarkamali/semango/internal/util"
	"github.com/omarkamali/semango/pkg/semango"
)

// OpenAIEmbedder implements the Embedder interface using OpenAI's API.
type OpenAIEmbedder struct {
	client     *openai.Client
	model      string
	dimension  int
	batchSize  int
	concurrent int
	limiter    *rate.Limiter
	mu         sync.RWMutex
}

// OpenAIConfig holds configuration for the OpenAI embedder.
type OpenAIConfig struct {
	APIKey     string  // Usually from OPENAI_API_KEY env var
	Model      string  // e.g., "text-embedding-3-large"
	BatchSize  int     // Number of texts to embed in a single API call
	Concurrent int     // Number of concurrent API calls
	RateLimit  float64 // Requests per second limit
	BaseURL    string  // Optional OpenAI API base URL override (e.g. for local endpoints)
}

// NewOpenAIEmbedder creates a new OpenAI embedding provider.
func NewOpenAIEmbedder(config OpenAIConfig) (*OpenAIEmbedder, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}
	if config.Model == "" {
		config.Model = "text-embedding-3-large" // Default model
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 48 // Default from spec
	}
	if config.Concurrent <= 0 {
		config.Concurrent = 4 // Default from spec
	}
	if config.RateLimit <= 0 {
		config.RateLimit = 10.0 // Conservative default: 10 requests per second
	}

	// Create client with optional base URL
	ocfg := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		ocfg.BaseURL = config.BaseURL
	}
	client := openai.NewClientWithConfig(ocfg)

	// Determine dimension based on model
	dimension := getModelDimension(config.Model)
	if dimension == 0 {
		return nil, fmt.Errorf("unknown model dimension for model: %s", config.Model)
	}

	return &OpenAIEmbedder{
		client:     client,
		model:      config.Model,
		dimension:  dimension,
		batchSize:  config.BatchSize,
		concurrent: config.Concurrent,
		limiter:    rate.NewLimiter(rate.Limit(config.RateLimit), 1),
	}, nil
}

// getModelDimension returns the embedding dimension for known OpenAI models.
func getModelDimension(model string) int {
	switch model {
	case "text-embedding-3-large":
		return 3072
	case "text-embedding-3-small":
		return 1536
	case "text-embedding-ada-002":
		return 1536
	case "text-embedding-nomic-embed-text-v1.5":
		return 768
	default:
		// For unknown models, return 0 to indicate we need to discover it
		return 0
	}
}

// Embed implements the Embedder interface.
func (oe *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	logger := util.FromContext(ctx)

	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	logger.Debug("Starting OpenAI embedding", "num_texts", len(texts), "model", oe.model)

	// Split texts into batches
	batches := oe.createBatches(texts)
	results := make([][]float32, len(texts))

	// Process batches with concurrency control
	sem := make(chan struct{}, oe.concurrent)
	errChan := make(chan error, len(batches))
	var wg sync.WaitGroup

	for i, batch := range batches {
		wg.Add(1)
		go func(batchIndex int, batchTexts []string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Rate limiting
			if err := oe.limiter.Wait(ctx); err != nil {
				errChan <- fmt.Errorf("rate limiting wait failed: %w", err)
				return
			}

			// Make API call with retries
			embeddings, err := oe.embedBatchWithRetry(ctx, batchTexts)
			if err != nil {
				errChan <- fmt.Errorf("batch %d failed: %w", batchIndex, err)
				return
			}

			// Store results in correct positions
			startIdx := batchIndex * oe.batchSize
			for j, embedding := range embeddings {
				if startIdx+j < len(results) {
					results[startIdx+j] = embedding
				}
			}
		}(i, batch)
	}

	// Wait for all batches to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			logger.Error("OpenAI embedding failed", "error", err)
			return nil, err
		}
	}

	logger.Debug("OpenAI embedding completed", "num_texts", len(texts), "num_results", len(results))
	return results, nil
}

// createBatches splits texts into batches of the configured size.
func (oe *OpenAIEmbedder) createBatches(texts []string) [][]string {
	var batches [][]string
	for i := 0; i < len(texts); i += oe.batchSize {
		end := i + oe.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batches = append(batches, texts[i:end])
	}
	return batches
}

// embedBatchWithRetry makes an API call to embed a batch of texts with retry logic.
func (oe *OpenAIEmbedder) embedBatchWithRetry(ctx context.Context, texts []string) ([][]float32, error) {
	logger := util.FromContext(ctx)

	maxRetries := 3
	baseDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := time.Duration(baseDelay) * time.Duration(1<<uint(attempt-1))
			logger.Debug("Retrying OpenAI API call", "attempt", attempt+1, "delay", delay)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		embeddings, err := oe.embedBatch(ctx, texts)
		if err == nil {
			return embeddings, nil
		}

		// Check if this is a retryable error
		if !isRetryableError(err) {
			return nil, fmt.Errorf("non-retryable error: %w", err)
		}

		logger.Warn("OpenAI API call failed, will retry", "attempt", attempt+1, "error", err)
	}

	return nil, fmt.Errorf("OpenAI API call failed after %d attempts", maxRetries)
}

// embedBatch makes a single API call to embed a batch of texts.
func (oe *OpenAIEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	req := openai.EmbeddingRequest{
		Input: texts,
		Model: openai.EmbeddingModel(oe.model),
	}

	resp, err := oe.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(resp.Data))
	}

	results := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		// Convert []float64 to []float32
		embedding := make([]float32, len(data.Embedding))
		for j, val := range data.Embedding {
			embedding[j] = float32(val)
		}
		results[i] = embedding
	}

	return results, nil
}

// isRetryableError determines if an error should trigger a retry.
func isRetryableError(err error) bool {
	// This is a simplified implementation. In practice, you'd want to check
	// for specific OpenAI error codes like rate limiting, temporary failures, etc.
	// The go-openai library might have specific error types we can check.
	return true // For now, retry all errors
}

// Dimension implements the Embedder interface.
func (oe *OpenAIEmbedder) Dimension() int {
	oe.mu.RLock()
	defer oe.mu.RUnlock()
	return oe.dimension
}

// Ensure OpenAIEmbedder implements the Embedder interface.
var _ semango.Embedder = (*OpenAIEmbedder)(nil)
