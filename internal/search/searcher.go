package search

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/go-faiss"
	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/ingest"
	"github.com/omarkamali/semango/internal/storage"
	"github.com/omarkamali/semango/internal/util"
)

// Searcher handles search operations using the real search implementation
type Searcher struct {
	config   *config.Config
	embedder ingest.Embedder
}

// Result represents a search result
type Result struct {
	Score         float64                `json:"score"`          // Combined score
	LexicalScore  float64                `json:"lexical_score"`  // BM25 relevance score
	SemanticScore float64                `json:"semantic_score"` // Cosine similarity score
	Modality      string                 `json:"modality"`
	Path          string                 `json:"path"`
	Text          string                 `json:"text"`
	Meta          map[string]string      `json:"meta,omitempty"`
	Highlights    map[string]interface{} `json:"highlights,omitempty"`
}

// Stats represents search statistics
type Stats struct {
	TotalDocuments int `json:"total_documents"`
	TotalChunks    int `json:"total_chunks"`
	IndexSize      int `json:"index_size_bytes"`
}

// NewSearcher creates a new searcher instance with real search capabilities
func NewSearcher(cfg *config.Config) (*Searcher, error) {
	// Initialize embedder (same logic as search command)
	var embedder ingest.Embedder
	prov := cfg.Embedding.Provider
	switch prov {
	case "openai", "": // default to openai
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, util.NewError("OpenAI API key is required but not found in OPENAI_API_KEY environment variable")
		}
		openCfg := ingest.OpenAIConfig{
			APIKey:     apiKey,
			Model:      cfg.Embedding.Model,
			BatchSize:  cfg.Embedding.BatchSize,
			Concurrent: cfg.Embedding.Concurrent,
		}
		e, err := ingest.NewOpenAIEmbedder(openCfg)
		if err != nil {
			return nil, util.WrapError(err, "Failed to create OpenAI embedder")
		}
		embedder = e
	case "local":
		if cfg.Embedding.LocalModelPath == "" {
			return nil, util.NewError("Local model path is required for local embedder provider")
		}
		localCfg := ingest.LocalEmbedderConfig{
			ModelPath: cfg.Embedding.LocalModelPath,
			CacheDir:  cfg.Embedding.ModelCacheDir,
			BatchSize: cfg.Embedding.BatchSize,
			MaxLength: 512, // Default max length
		}
		// Validate configuration
		if err := ingest.ValidateModelConfig(localCfg); err != nil {
			return nil, util.WrapError(err, "Invalid local embedder configuration")
		}
		e, err := ingest.NewLocalEmbedder(localCfg)
		if err != nil {
			return nil, util.WrapError(err, "Failed to create local embedder")
		}
		embedder = e
	default:
		return nil, util.NewError(fmt.Sprintf("Unsupported embedder provider: %s. Supported providers: openai, local", prov))
	}

	return &Searcher{
		config:   cfg,
		embedder: embedder,
	}, nil
}

// Search performs a real search query using the existing search implementation
func (s *Searcher) Search(ctx context.Context, query string, topK int) ([]Result, error) {
	slog.Info("Performing hybrid search", "query", query, "top_k", topK)

	// Perform lexical search
	bleveIdx, err := storage.OpenOrCreateBleveIndex(s.config.Lexical.IndexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Bleve index: %w", err)
	}
	defer bleveIdx.Close()

	lexicalHits, err := bleveIdx.SearchText(query, topK*2) // Get more for better fusion
	if err != nil {
		return nil, fmt.Errorf("lexical search failed: %w", err)
	}

	slog.Debug("Lexical search results", "query", query, "hits", len(lexicalHits))
	for i, hit := range lexicalHits {
		if i < 3 { // Log first 3 hits
			slog.Debug("Lexical hit", "rank", i+1, "id", hit.ID, "score", hit.Score)
		}
	}

	// Perform vector search
	queryEmbedding, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Open vector index
	faissPath := filepath.Join("semango", "index", "faiss.index")
	vecIdx, err := storage.NewFaissVectorIndex(ctx, faissPath, s.embedder.Dimension(), faiss.MetricInnerProduct)
	if err != nil {
		return nil, fmt.Errorf("failed to open vector index: %w", err)
	}
	defer vecIdx.Close()

	vecResults, err := vecIdx.Search(ctx, queryEmbedding[0], topK*2)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	slog.Debug("Vector search results", "query", query, "hits", len(vecResults))
	for i, result := range vecResults {
		if i < 3 { // Log first 3 hits
			slog.Debug("Vector hit", "rank", i+1, "id", result.ID, "score", result.Score)
		}
	}

	// Create rank maps for RRF
	lexicalRanks := make(map[string]int)
	semanticRanks := make(map[string]int)

	// Create rank maps (only needed for RRF)
	for i, hit := range lexicalHits {
		lexicalRanks[hit.ID] = i + 1 // Rank starts from 1
	}

	for i, result := range vecResults {
		semanticRanks[result.ID] = i + 1 // Rank starts from 1
	}

	slog.Debug("Raw score ranges",
		"lexical_hits", len(lexicalHits),
		"semantic_hits", len(vecResults))

	// Collect all unique chunk IDs
	allChunkIDs := make(map[string]bool)
	for _, hit := range lexicalHits {
		allChunkIDs[hit.ID] = true
	}
	// Deduplicate vector results
	seenVectorIDs := make(map[string]bool)
	for _, result := range vecResults {
		if !seenVectorIDs[result.ID] {
			allChunkIDs[result.ID] = true
			seenVectorIDs[result.ID] = true
		}
	}

	// Build final results with proper relevance scoring
	var finalResults []Result

	slog.Debug("Processing chunks", "total_unique_chunks", len(allChunkIDs))

	for chunkID := range allChunkIDs {
		// Get document from Bleve to extract text and metadata
		doc, err := bleveIdx.GetDocument(chunkID)
		if err != nil || doc == nil {
			slog.Warn("Could not retrieve document", "chunk_id", chunkID, "error", err)
			continue
		}

		var text string
		meta := make(map[string]string)
		var path string

		// Extract fields from Bleve document
		for _, field := range doc.Fields {
			switch field.Name() {
			case "text":
				text = string(field.Value())
			case "path":
				path = string(field.Value())
			default:
				// Handle flattened meta fields e.g., "meta.path", "meta.source"
				if strings.HasPrefix(field.Name(), "meta.") {
					key := strings.TrimPrefix(field.Name(), "meta.")
					meta[key] = string(field.Value())
				}
			}
		}

		// Fallback for path if not found in document fields
		if path == "" && meta != nil {
			path = meta["path"]
		}

		// Calculate combined score using proper relevance scoring
		var finalScore float64

		// Get raw scores for this chunk
		var lexicalScore float64 = 0.0
		var semanticScore float64 = 0.0

		// Check if this chunk was found by lexical search
		foundInLexical := false
		foundInSemantic := false

		// Find lexical score (BM25 relevance)
		for _, hit := range lexicalHits {
			if hit.ID == chunkID {
				lexicalScore = hit.Score
				foundInLexical = true
				break
			}
		}

		// Find semantic score (cosine similarity)
		for _, result := range vecResults {
			if result.ID == chunkID {
				semanticScore = float64(result.Score)
				foundInSemantic = true
				break
			}
		}

		slog.Debug("Chunk analysis",
			"chunk_id", chunkID,
			"found_lexical", foundInLexical,
			"found_semantic", foundInSemantic,
			"raw_lexical", lexicalScore,
			"raw_semantic", semanticScore)

		// Normalize BM25 score consistently across all searches using standard formula
		// BM25 can theoretically go to infinity, so we use: score / (score + 1)
		// This maps [0, ∞) to [0, 1) consistently
		normalizedLexical := lexicalScore / (lexicalScore + 1.0)

		// Semantic score is already 0-1 (cosine similarity)
		normalizedSemantic := semanticScore

		// Apply hybrid fusion using consistently normalized scores
		switch s.config.Hybrid.Fusion {
		case "rrf":
			// Reciprocal Rank Fusion using actual ranks
			k := 60.0
			rrfScore := 0.0

			if lexicalRank, hasLexical := lexicalRanks[chunkID]; hasLexical {
				rrfScore += s.config.Hybrid.LexicalWeight / (k + float64(lexicalRank))
			}

			if semanticRank, hasSemantic := semanticRanks[chunkID]; hasSemantic {
				rrfScore += s.config.Hybrid.VectorWeight / (k + float64(semanticRank))
			}

			finalScore = rrfScore

		case "linear":
			// Linear combination of consistently normalized scores
			finalScore = (normalizedLexical * s.config.Hybrid.LexicalWeight) +
				(normalizedSemantic * s.config.Hybrid.VectorWeight)

		default:
			// Default to linear combination
			finalScore = (normalizedLexical * s.config.Hybrid.LexicalWeight) +
				(normalizedSemantic * s.config.Hybrid.VectorWeight)
		}

		slog.Debug("Score calculation",
			"chunk_id", chunkID,
			"raw_lexical", lexicalScore,
			"raw_semantic", semanticScore,
			"norm_lexical", normalizedLexical,
			"norm_semantic", normalizedSemantic,
			"weights", fmt.Sprintf("lex=%.1f sem=%.1f", s.config.Hybrid.LexicalWeight, s.config.Hybrid.VectorWeight),
			"final_score", finalScore,
			"fusion", s.config.Hybrid.Fusion)

		// Create highlights only for lexical matches
		var highlights map[string]interface{}
		if _, hasLexical := lexicalRanks[chunkID]; hasLexical {
			highlights = s.createHighlights(text, query)
		}

		result := Result{
			Score:         finalScore,
			LexicalScore:  lexicalScore,
			SemanticScore: semanticScore,
			Modality:      getModality(meta["modality"], path),
			Path:          path,
			Text:          text, // Complete chunk content
			Meta:          meta,
			Highlights:    highlights,
		}

		finalResults = append(finalResults, result)
	}

	// Sort by final score (descending)
	for i := 0; i < len(finalResults)-1; i++ {
		for j := i + 1; j < len(finalResults); j++ {
			if finalResults[i].Score < finalResults[j].Score {
				finalResults[i], finalResults[j] = finalResults[j], finalResults[i]
			}
		}
	}

	// Limit to topK
	if len(finalResults) > topK {
		finalResults = finalResults[:topK]
	}

	slog.Info("Search completed", "total_results", len(finalResults), "lexical_hits", len(lexicalHits), "vector_hits", len(vecResults))
	return finalResults, nil
}

// Helper method to get representation by ID (this would need to be implemented)
func (s *Searcher) getRepresentationByID(id string) (ingest.Representation, bool) {
	// TODO: This would need access to the representation store
	// For now, return empty representation
	return ingest.Representation{}, false
}

// createHighlights creates highlight information for lexical matches
func (s *Searcher) createHighlights(text, query string) map[string]interface{} {
	highlights := make(map[string]interface{})

	// Simple case-insensitive highlighting
	queryLower := strings.ToLower(query)
	textLower := strings.ToLower(text)

	var matches []map[string]int
	start := 0

	for {
		index := strings.Index(textLower[start:], queryLower)
		if index == -1 {
			break
		}

		actualStart := start + index
		actualEnd := actualStart + len(query)

		matches = append(matches, map[string]int{
			"start": actualStart,
			"end":   actualEnd,
		})

		start = actualEnd
	}

	if len(matches) > 0 {
		highlights["text"] = matches
	}

	return highlights
}

// combineScores combines lexical and semantic scores based on fusion strategy
func combineScores(lexicalScore, semanticScore float64, fusion string) float64 {
	switch fusion {
	case "rrf": // Reciprocal Rank Fusion
		// For RRF, we'd need ranks, but for simplicity, use weighted average
		return lexicalScore + semanticScore
	case "linear":
		return lexicalScore + semanticScore
	default:
		return lexicalScore + semanticScore
	}
}

// GetStats returns real search index statistics
func (s *Searcher) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}

	// Get Bleve stats - estimate based on search results
	bleveIdx, err := storage.OpenOrCreateBleveIndex(s.config.Lexical.IndexPath)
	if err == nil {
		defer bleveIdx.Close()
		// Estimate document count by doing a broad search
		if hits, err := bleveIdx.SearchText("*", 1000); err == nil {
			stats.TotalDocuments = len(hits)
		}
	}

	// Get FAISS stats
	faissPath := filepath.Join("semango", "index", "faiss.index")
	if info, err := os.Stat(faissPath); err == nil {
		stats.IndexSize = int(info.Size())
	}

	// Estimate chunks (rough approximation)
	stats.TotalChunks = stats.TotalDocuments * 3 // Rough estimate

	return stats, nil
}

// Helper functions
func getModality(modalityMeta, path string) string {
	if modalityMeta != "" {
		return modalityMeta
	}

	// Infer from file extension
	ext := filepath.Ext(path)
	switch ext {
	case ".go", ".js", ".ts", ".py", ".java", ".cpp", ".c", ".h":
		return "code"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg":
		return "image"
	case ".mp3", ".wav", ".flac", ".m4a":
		return "audio"
	case ".pdf":
		return "pdf"
	default:
		return "text"
	}
}
