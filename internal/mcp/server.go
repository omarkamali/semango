package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/search"
)

// Server wraps the MCP server and its dependencies
type Server struct {
	cfg      *config.Config
	searcher *search.Searcher
	mcp      *server.MCPServer
}

// NewServer creates a new MCP server instance
func NewServer(cfg *config.Config, searcher *search.Searcher) *Server {
	s := server.NewMCPServer(
		"🥭 Semango",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	srv := &Server{
		cfg:      cfg,
		searcher: searcher,
		mcp:      s,
	}

	srv.registerTools()
	return srv
}

func (s *Server) registerTools() {
	// Search Tool
	searchTool := mcp.NewTool("search",
		mcp.WithDescription("Search the indexed codebase and documentation using hybrid search."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query in natural language."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 5)."),
		),
	)

	s.mcp.AddTool(searchTool, s.handleSearch)

	// Stats Tool
	statsTool := mcp.NewTool("get_stats",
		mcp.WithDescription("Get indexing statistics like document counts."),
	)

	s.mcp.AddTool(statsTool, s.handleStats)
}

func (s *Server) handleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, _ := request.RequireString("query")
	limit := request.GetInt("limit", 5)

	slog.Info("MCP Search request", "query", query, "limit", limit)

	results, err := s.searcher.Search(ctx, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No results found."), nil
	}

	formatted := "Search Results:\n\n"
	for _, res := range results {
		formatted += fmt.Sprintf("- [%s](%s) (Score: %.4f)\n", res.Path, res.Path, res.Score)
		if res.Text != "" {
			snippet := res.Text
			if len(snippet) > 200 {
				snippet = snippet[:197] + "..."
			}
			formatted += fmt.Sprintf("  Snippet: %s\n", snippet)
		}
		formatted += "\n"
	}

	return mcp.NewToolResultText(formatted), nil
}

func (s *Server) handleStats(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stats, err := s.searcher.GetStats(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get stats: %v", err)), nil
	}

	formatted := fmt.Sprintf("🥭 Semango Stats:\n- Documents (BM25): %d\n- Documents (Vector): %d\n", 
		stats.LexicalCount, stats.VectorCount)
	
	return mcp.NewToolResultText(formatted), nil
}

// ServeStdio starts the server using stdin/stdout transport
func (s *Server) ServeStdio() error {
	slog.Info("Starting 🥭 Semango MCP server (stdio)")
	return server.ServeStdio(s.mcp)
}

// ServeSSE starts the server using SSE transport on the given address
func (s *Server) ServeSSE(addr string) error {
	slog.Info("Starting 🥭 Semango MCP server (SSE)", "addr", addr)
	sseServer := server.NewSSEServer(s.mcp, server.WithBaseURL("http://"+addr))
	return http.ListenAndServe(addr, sseServer)
}
