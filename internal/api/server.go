package api

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/omneity-labs/semango/internal/config"
	"github.com/omneity-labs/semango/internal/search"
	"github.com/omneity-labs/semango/internal/util"
)

//go:embed all:ui
var uiFiles embed.FS

// Server represents the HTTP API server
type Server struct {
	config   *config.Config
	searcher *search.Searcher
	router   *gin.Engine
	logger   *slog.Logger
	uiFS     fs.FS
}

// SearchRequest represents the search API request
type SearchRequest struct {
	Query  string `json:"query" binding:"required"`
	TopK   int    `json:"top_k,omitempty"`
	Filter string `json:"filter,omitempty"`
}

// SearchResponse represents the search API response
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Query   string         `json:"query"`
	TopK    int            `json:"top_k"`
	Took    string         `json:"took"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Rank          int                    `json:"rank"`
	Score         float64                `json:"score"`
	LexicalScore  float64                `json:"lexical_score"`
	SemanticScore float64                `json:"semantic_score"`
	Modality      string                 `json:"modality"`
	Document      DocumentInfo           `json:"document"`
	Chunk         string                 `json:"chunk"`
	Highlights    map[string]interface{} `json:"highlights,omitempty"`
}

// DocumentInfo represents document metadata
type DocumentInfo struct {
	Path string            `json:"path"`
	Meta map[string]string `json:"meta,omitempty"`
}

// NewServer creates a new API server instance
func NewServer(config *config.Config, searcher *search.Searcher, uiFS fs.FS) *Server {
	// Use embedded UI files if no external FS provided
	if uiFS == nil {
		subFS, err := fs.Sub(uiFiles, "ui")
		if err == nil {
			uiFS = subFS
		}
	}

	return &Server{
		config:   config,
		searcher: searcher,
		uiFS:     uiFS,
	}
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// API routes
	api := s.router.Group("/api/v1")
	{
		api.POST("/search", s.handleSearch)
		api.GET("/health", s.handleHealth)
		api.GET("/stats", s.handleStats)
	}

	// Serve embedded UI
	s.setupUIRoutes()
}

// setupUIRoutes configures routes for serving the embedded React UI
func (s *Server) setupUIRoutes() {
	if s.uiFS == nil {
		s.logger.Warn("UI filesystem not provided, serving fallback page")
		s.router.GET("/", s.handleFallbackUI)
		return
	}

	// Serve static assets from /assets/ path
	assetsFS, err := fs.Sub(s.uiFS, "assets")
	if err == nil {
		s.router.StaticFS("/assets", http.FS(assetsFS))
	}

	// Serve other static files from root
	s.router.GET("/vite.svg", func(c *gin.Context) {
		file, err := s.uiFS.Open("vite.svg")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer file.Close()
		c.Header("Content-Type", "image/svg+xml")
		io.Copy(c.Writer, file)
	})

	// Serve index.html for root and SPA routes
	s.router.NoRoute(func(c *gin.Context) {
		// If it's an API route, return 404
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
			return
		}

		// For all other routes, serve the React app
		indexFile, err := s.uiFS.Open("index.html")
		if err != nil {
			s.handleFallbackUI(c)
			return
		}
		defer indexFile.Close()

		c.Header("Content-Type", "text/html")
		c.Status(http.StatusOK)
		io.Copy(c.Writer, indexFile)
	})

	// Serve index.html for root
	s.router.GET("/", func(c *gin.Context) {
		indexFile, err := s.uiFS.Open("index.html")
		if err != nil {
			s.handleFallbackUI(c)
			return
		}
		defer indexFile.Close()

		c.Header("Content-Type", "text/html")
		c.Status(http.StatusOK)
		io.Copy(c.Writer, indexFile)
	})
}

// handleFallbackUI serves a simple fallback page when UI is not available
func (s *Server) handleFallbackUI(c *gin.Context) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Semango Search</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 600px; margin: 0 auto; }
        .search-box { width: 100%; padding: 10px; margin: 20px 0; }
        .result { margin: 20px 0; padding: 15px; border: 1px solid #ddd; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Semango Search</h1>
        <p>UI not available. Use the API directly:</p>
        <pre>curl -X POST http://localhost:8181/api/v1/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"your search query", "top_k": 5}'</pre>
    </div>
</body>
</html>`
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html)
}

// handleSearch handles the search API endpoint
func (s *Server) handleSearch(c *gin.Context) {
	start := time.Now()

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default top_k if not provided
	if req.TopK <= 0 {
		req.TopK = 10
	}
	if req.TopK > 100 {
		req.TopK = 100 // Limit to prevent abuse
	}

	// Perform search
	results, err := s.searcher.Search(c.Request.Context(), req.Query, req.TopK)
	if err != nil {
		s.logger.Error("Search failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}

	// Convert results to API format
	apiResults := make([]SearchResult, len(results))
	for i, result := range results {
		apiResults[i] = SearchResult{
			Rank:          i + 1,
			Score:         result.Score,
			LexicalScore:  result.LexicalScore,
			SemanticScore: result.SemanticScore,
			Modality:      result.Modality,
			Document: DocumentInfo{
				Path: result.Path,
				Meta: result.Meta,
			},
			Chunk:      result.Text,
			Highlights: result.Highlights,
		}
	}

	response := SearchResponse{
		Results: apiResults,
		Query:   req.Query,
		TopK:    req.TopK,
		Took:    time.Since(start).String(),
	}

	c.JSON(http.StatusOK, response)
}

// handleHealth handles the health check endpoint
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().UTC(),
	})
}

// handleStats handles the stats endpoint
func (s *Server) handleStats(c *gin.Context) {
	stats, err := s.searcher.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	// Set Gin mode to release by default
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	s.router = router
	s.logger = util.Logger
	s.setupRoutes()

	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	slog.Info("Starting HTTP server", "address", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	slog.Info("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
