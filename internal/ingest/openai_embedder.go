package ingest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/omneity-labs/semango/internal/util/log"
	"github.com/omneity-labs/semango/pkg/semango"
	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/time/rate"
)

const (
	defaultOpenAIRetries    = 3
	defaultOpenAIRetryDelay = 2 * time.Second
	defaultOpenAIQPS        = 5 // Default QPS for client-side rate limiting
)

// OpenAIEmbedderConfig holds configuration for the OpenAI embedding provider.
type OpenAIEmbedderConfig struct {
	APIKey       string
	Model        string
	BatchSize    int
	Concurrency  int
	Retries      int
	RetryDelay   time.Duration
	RateLimitQPS float64
	// OrganizationID string // Optional OpenAI Organization ID
}

// Compile-time check to ensure OpenAIEmbedder implements semango.Embedder
var _ semango.Embedder = (*OpenAIEmbedder)(nil)

// OpenAIEmbedder implements the semango.Embedder interface using the OpenAI API.
type OpenAIEmbedder struct {
	client    *openai.Client
	config    OpenAIEmbedderConfig
	dimension int
	limiter   *rate.Limiter
}

// NewOpenAIEmbedder creates a new OpenAIEmbedder.
func NewOpenAIEmbedder(ctx context.Context, config OpenAIEmbedderConfig) (*OpenAIEmbedder, error) {
	logger := log.FromContext(ctx)

	if config.APIKey == "" {
		config.APIKey = os.Getenv("OPENAI_API_KEY")
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not provided and OPENAI_API_KEY environment variable not set")
	}
	if config.Model == "" {
		return nil, fmt.Errorf("OpenAI model not specified in config")
	}

	// Apply defaults for unspecified config values
	if config.BatchSize <= 0 {
		config.BatchSize = 32 // A common default
		logger.Debug("OpenAIEmbedder: BatchSize not set, defaulting", "value", config.BatchSize)
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 4 // A common default
		logger.Debug("OpenAIEmbedder: Concurrency not set, defaulting", "value", config.Concurrency)
	}
	if config.Retries < 0 { // Allow 0 for no retries
		config.Retries = defaultOpenAIRetries
		logger.Debug("OpenAIEmbedder: Retries not set, defaulting", "value", config.Retries)
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = defaultOpenAIRetryDelay
		logger.Debug("OpenAIEmbedder: RetryDelay not set, defaulting", "value", config.RetryDelay)
	}
	if config.RateLimitQPS <= 0 {
		config.RateLimitQPS = defaultOpenAIQPS
		logger.Debug("OpenAIEmbedder: RateLimitQPS not set, defaulting", "value", config.RateLimitQPS)
	}

	var dim int
	switch config.Model {
	case "text-embedding-ada-002":
		dim = 1536
	case "text-embedding-3-small":
		dim = 1536
	case "text-embedding-3-large":
		dim = 3072
	default:
		// Attempt to infer from model name if it contains dimensions like '...-1536' or '...-3072'
		logger.Warn("Unknown OpenAI model specified, attempting to infer dimension.", "model", config.Model)
		if strings.Contains(config.Model, "-3072") {
			dim = 3072
		} else if strings.Contains(config.Model, "-1536") {
			dim = 1536
		} else if strings.Contains(config.Model, "ada") {
			dim = 1536 // Default for older ada, actual was 1024 for some, 1536 for ada-002
		} else {
			return nil, fmt.Errorf("unknown OpenAI model '%s' and could not infer dimension; please add to mapping or use a model name with explicit dimension", config.Model)
		}
		logger.Info("Inferred dimension for model", "model", config.Model, "dimension", dim)
	}

	clientConfig := openai.DefaultConfig(config.APIKey)
	// if config.OrganizationID != "" {
	// 	clientConfig.OrgID = config.OrganizationID
	// }
	client := openai.NewClientWithConfig(clientConfig)
	limiter := rate.NewLimiter(rate.Limit(config.RateLimitQPS), config.Concurrency)

	logger.Info("OpenAI Embedder initialized",
		"model", config.Model,
		"dimension", dim,
		"batch_size", config.BatchSize,
		"concurrency", config.Concurrency,
		"qps_limit", config.RateLimitQPS,
		"retries", config.Retries,
		"retry_delay", config.RetryDelay.String(),
	)
	return &OpenAIEmbedder{
		client:    client,
		config:    config,
		dimension: dim,
		limiter:   limiter,
	}, nil
}

// Dimension returns the embedding dimension for the configured model.
func (e *OpenAIEmbedder) Dimension() int {
	if e == nil {
		// This case should ideally not happen if constructor is used correctly.
		// Log an error or panic if it indicates a programming error.
		return 0
	}
	return e.dimension
}

// Embed generates embeddings for a slice of texts.
// It handles batching, concurrency, rate limiting, and retries.
func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	logger := log.FromContext(ctx)
	if e == nil || e.client == nil {
		return nil, fmt.Errorf("OpenAIEmbedder not initialized")
	}
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	totalTexts := len(texts)
	allEmbeddings := make([][]float32, totalTexts)

	// Channel to distribute text indices to workers
	jobs := make(chan int, totalTexts)
	for i := 0; i < totalTexts; i++ {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	var firstError error
	mu := &sync.Mutex{}
	ctx, cancel := context.WithCancel(ctx) // Context for early exit on error
	defer cancel()

	numWorkers := e.config.Concurrency
	if totalTexts < numWorkers {
		numWorkers = totalTexts
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			currentBatch := make([]string, 0, e.config.BatchSize)
			originalIndices := make([]int, 0, e.config.BatchSize)

			processBatch := func() error {
				if len(currentBatch) == 0 {
					return nil
				}

				logger.Debug("Processing batch", "worker_id", workerID, "batch_size", len(currentBatch), "texts_in_batch", len(currentBatch))

				var attemptErr error
				for attempt := 0; attempt <= e.config.Retries; attempt++ {
					if ctx.Err() != nil { // Check for context cancellation from other goroutines
						logger.Debug("Context cancelled, worker exiting batch processing", "worker_id", workerID)
						return ctx.Err()
					}

					if err := e.limiter.Wait(ctx); err != nil {
						logger.Error("Rate limiter wait error", "worker_id", workerID, "error", err)
						return fmt.Errorf("rate limiter error: %w", err) // Critical error, don't retry this step
					}

					apiModel := openai.EmbeddingModel(e.config.Model)
					if apiModel == "" {
						// This should have been caught in NewOpenAIEmbedder, but as a safeguard:
						return fmt.Errorf("OpenAI model name is empty in config")
					}

					req := openai.EmbeddingRequest{
						Input: currentBatch,
						Model: apiModel,
					}

					logger.Debug("Sending request to OpenAI API", "worker_id", workerID, "num_texts", len(currentBatch), "model", e.config.Model)
					resp, err := e.client.CreateEmbeddings(ctx, req)
					if err == nil {
						if len(resp.Data) != len(currentBatch) {
							errMsg := fmt.Sprintf("OpenAI API returned %d embeddings for %d inputs", len(resp.Data), len(currentBatch))
							logger.Error("Embeddings count mismatch from OpenAI", "worker_id", workerID, "error", errMsg, "batch_size", len(currentBatch))
							attemptErr = fmt.Errorf(errMsg)
							// This is a significant issue; decide if retry is appropriate or if it should fail fast.
							// For now, we continue to retry, but this might warrant a different strategy.
							continue
						}
						mu.Lock()
						for i, data := range resp.Data {
							// The API is expected to return embeddings in the same order as the input.
							// The `data.Index` field in the response refers to the original index within that batch request.
							allEmbeddings[originalIndices[i]] = data.Embedding
						}
						mu.Unlock()
						attemptErr = nil // Success
						logger.Debug("Successfully embedded batch", "worker_id", workerID, "batch_size", len(currentBatch))
						break // Break retry loop
					}
					attemptErr = err
					logger.Warn("Failed to create embeddings, retrying", "worker_id", workerID, "error", err, "attempt", attempt+1, "max_retries", e.config.Retries, "batch_size", len(currentBatch))
					if attempt < e.config.Retries {
						time.Sleep(e.config.RetryDelay)
					} else {
						logger.Error("Failed to create embeddings after all retries", "worker_id", workerID, "error", err, "batch_size", len(currentBatch))
					}
				}
				// Clear batch for next set of texts for this worker
				currentBatch = currentBatch[:0]
				originalIndices = originalIndices[:0]
				return attemptErr
			}

			for jobIndex := range jobs {
				if ctx.Err() != nil {
					return // Context cancelled, stop processing jobs
				}
				currentBatch = append(currentBatch, texts[jobIndex])
				originalIndices = append(originalIndices, jobIndex)

				if len(currentBatch) >= e.config.BatchSize {
					if err := processBatch(); err != nil {
						mu.Lock()
						if firstError == nil { // Record only the first error
							firstError = err
							cancel() // Signal other workers to stop
						}
						mu.Unlock()
						return // Exit worker on error
					}
				}
			}
			// Process any remaining items in the batch for this worker
			if len(currentBatch) > 0 && ctx.Err() == nil { // Check ctx.Err() again before final batch
				if err := processBatch(); err != nil {
					mu.Lock()
					if firstError == nil {
						firstError = err
						cancel()
					}
					mu.Unlock()
					// return not strictly needed as loop is done, but for clarity
				}
			}
		}(i)
	}

	wg.Wait()

	if firstError != nil {
		return nil, fmt.Errorf("OpenAIEmbedder.Embed failed: %w", firstError)
	}

	// Sanity check: ensure all embeddings were populated.
	// This can happen if job distribution or error handling logic is flawed.
	for i, emb := range allEmbeddings {
		if emb == nil && firstError == nil { // If no general error reported, but an embedding is missing
			logger.Error("Nil embedding found post-processing without a reported error", "index", i, "text_snippet", texts[i][:min(20, len(texts[i]))])
			// This indicates a bug in the concurrent processing logic.
			return nil, fmt.Errorf("internal error: nil embedding for text index %d without explicit error", i)
		}
	}

	logger.Info("Successfully embedded all texts with OpenAI", "num_texts", totalTexts, "model", e.config.Model)
	return allEmbeddings, nil
}

// min utility, can be moved to a util package if used elsewhere
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
