package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/omarkamali/semango/internal/api"
	"github.com/omarkamali/semango/internal/cli"
	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/ingest"
	"github.com/omarkamali/semango/internal/mcp"
	"github.com/omarkamali/semango/internal/pipeline"
	"github.com/omarkamali/semango/internal/search"
	"github.com/omarkamali/semango/internal/storage"
	"github.com/omarkamali/semango/internal/util"
	"github.com/omarkamali/semango/pkg/types"
	"github.com/spf13/cobra"
)

// Version information set by ldflags during build
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var AppConfig *config.Config // Global config instance

var rootCmd = &cobra.Command{
	Use:   "semango",
	Short: "🥭 Semango is a semantic search engine.",
	Long:  `A fast and flexible semantic search engine capable of indexing and searching various file types.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		_ = util.Logger // Ensure logger is initialized

		verbose, _ := cmd.Flags().GetBool("verbose")
		if verbose {
			util.SetLogLevel(slog.LevelDebug)
		}

		// Skip configuration loading for commands that don't need it
		skipConfigCommands := map[string]bool{
			"init":         true,
			"install":      true,
			"version":      true,
			"models":       true,
			"_pdf-extract": true,
			"help":         true,
			"completion":   true,
		}

		isSkip := false
		for curr := cmd; curr != nil; curr = curr.Parent() {
			if skipConfigCommands[curr.Name()] {
				isSkip = true
				break
			}
		}

		if isSkip {
			slog.Debug("Skipping configuration loading", "command", cmd.Name())
			return nil
		}

		configPath, _ := cmd.Flags().GetString("config")
		slog.Debug("Loading configuration", "path", configPath)
		loadedCfg, err := config.Load(configPath, config.DefaultCueSchemaPath)
		if err != nil {
			wrappedErr := util.WrapError(err, "Failed to load configuration", slog.String("config_path", configPath))
			var unknownFieldErr *config.ErrUnknownField
			if errors.As(err, &unknownFieldErr) {
				util.LogError(util.Logger, util.WrapError(wrappedErr, "Configuration contains unknown fields. Exit 78."))
				os.Exit(78)
			} else {
				util.LogError(util.Logger, wrappedErr)
				os.Exit(1)
			}
		}
		AppConfig = loadedCfg // Store loaded config globally
		slog.Info("Configuration loaded and validated successfully")
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		slog.Info("Welcome to 🥭 Semango! Use -h or --help for available commands.")
	},
}

// Internal command used for killable PDF extraction.
// Streams JSON objects to stdout; must never log to stdout.
var pdfExtractCmd = &cobra.Command{
	Use:    "_pdf-extract",
	Hidden: true,
	Short:  "(internal) Extract PDF text",
	RunE: func(cmd *cobra.Command, args []string) error {
		absPath, _ := cmd.Flags().GetString("abs")
		if absPath == "" {
			return fmt.Errorf("missing --abs")
		}
		enc := json.NewEncoder(os.Stdout)
		return ingest.ExtractPDFPages(absPath, func(p ingest.PDFExtractPage) error {
			return enc.Encode(p)
		})
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Semango configuration file.",
	Long:  `Creates a new semango.yml configuration file in the current directory with default values.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("file")
		if err := config.WriteDefaultConfig(configPath); err != nil {
			wrappedErr := util.WrapError(err, "Failed to write default config", slog.String("path", configPath))
			util.LogError(util.Logger, wrappedErr)
			return wrappedErr // Return the wrapped error for cobra to handle
		}
		slog.Info("Default configuration written", "path", configPath)
		return nil
	},
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the Semango search server.",
	Long:  `Starts the HTTP server with REST API and web UI for searching indexed content.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if AppConfig == nil {
			cfgErr := util.NewError("Configuration not loaded before server command")
			util.LogError(util.Logger, cfgErr)
			return cfgErr
		}

		slog.Info("Starting Semango server...", "host", AppConfig.Server.Host, "port", AppConfig.Server.Port)

		// Initialize searcher with real search capabilities
		searcher, err := search.NewSearcher(AppConfig)
		if err != nil {
			wrappedErr := util.WrapError(err, "Failed to initialize searcher")
			util.LogError(util.Logger, wrappedErr)
			return wrappedErr
		}

		// Create API server with nil UI filesystem (will use fallback)
		server := api.NewServer(AppConfig, searcher, nil)

		// Create context for graceful shutdown
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Requirement 1 & 2: Automatic indexing and reconciliation
		go func() {
			// Check if index exists by getting stats
			stats, err := searcher.GetStats(ctx)
			if err != nil {
				slog.Error("Failed to check index stats", "error", err)
			}

			// If no documents, trigger initial indexing
			if stats != nil && stats.LexicalCount == 0 {
				slog.Info("No index found, starting initial indexing...")
			}

			mgr := pipeline.NewManager(AppConfig, searcher.Embedder())
			if err := mgr.RunReconciliation(ctx); err != nil {
				slog.Error("Initial reconciliation failed", "error", err)
			}

			// Periodic reconciliation
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					slog.Info("Running periodic reconciliation...")
					if err := mgr.RunReconciliation(ctx); err != nil {
						slog.Error("Periodic reconciliation failed", "error", err)
					}
				}
			}
		}()

		// Handle shutdown signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			slog.Info("Received shutdown signal, stopping server...")
			cancel()
		}()

		// Start server
		if err := server.Start(ctx); err != nil {
			wrappedErr := util.WrapError(err, "Server failed to start")
			util.LogError(util.Logger, wrappedErr)
			return wrappedErr
		}

		slog.Info("Server stopped gracefully")
		return nil
	},
}

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index files based on the configuration.",
	Long:  `Crawls the filesystem according to the include/exclude patterns in semango.yml and processes files for indexing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if AppConfig == nil {
			return util.NewError("Configuration not loaded before index command")
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			slog.Info("Received shutdown signal, stopping indexing...")
			cancel()
		}()

		embedder, err := ingest.NewEmbedderFromConfig(AppConfig.Embedding)
		if err != nil {
			return util.WrapError(err, "Failed to initialize embedder")
		}

		mgr := pipeline.NewManager(AppConfig, embedder)
		if err := mgr.RunReconciliation(ctx); err != nil {
			if ctx.Err() != nil {
				slog.Info("Indexing cancelled.")
				return nil
			}
			return util.WrapError(err, "Reconciliation failed")
		}

		slog.Info("Indexing and reconciliation process completed.")
		return nil
	},
}

var indexStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show indexing statistics.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if AppConfig == nil {
			return util.NewError("Configuration not loaded before stats command")
		}

		searcher, err := search.NewSearcher(AppConfig)
		if err != nil {
			return util.WrapError(err, "Failed to initialize searcher")
		}

		stats, err := searcher.GetStats(context.Background())
		if err != nil {
			return util.WrapError(err, "Failed to get index stats")
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	},
}

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search indexed text content.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if AppConfig == nil {
			cfgErr := util.NewError("Configuration not loaded before search command")
			util.LogError(util.Logger, cfgErr)
			return cfgErr
		}
		query := args[0]
		size := 10 // TODO: Make this configurable via flag or config
		bleveIdx, err := storage.OpenOrCreateBleveIndex(AppConfig.Lexical.IndexPath)
		if err != nil {
			wrappedErr := util.WrapError(err, "Failed to open Bleve index", slog.String("path", AppConfig.Lexical.IndexPath))
			util.LogError(util.Logger, wrappedErr)
			return wrappedErr
		}
		defer bleveIdx.Close()

		// Perform lexical search
		lexHits, err := bleveIdx.SearchText(query, size)
		if err != nil {
			return util.WrapError(err, "Lexical search failed")
		}

		// Vector index path same as indexing
		faissPath := filepath.Join(filepath.Dir(AppConfig.Lexical.IndexPath), "faiss.index")
		// Initialize embedder (same logic as ingest.NewEmbedderFromConfig)
		embedder, err := ingest.NewEmbedderFromConfig(AppConfig.Embedding)
		if err != nil {
			return util.WrapError(err, "Failed to initialize embedder")
		}

		queryVecs, err := embedder.Embed(context.Background(), []string{query})
		if err != nil {
			return util.WrapError(err, "Embedding query failed")
		}
		vecIdx, err := storage.NewFaissVectorIndex(context.Background(), faissPath, embedder.Dimension(), types.MetricInnerProduct)
		if err != nil {
			return util.WrapError(err, "Opening vector index failed")
		}
		defer vecIdx.Close()
		vecResults, _ := vecIdx.Search(context.Background(), queryVecs[0], size)

		// Build JSON structure
		type hit struct {
			ID    string  `json:"id"`
			Score float32 `json:"score"`
			Text  string  `json:"text"`
		}
		type output struct {
			Lexical []hit `json:"lexical"`
			Vector  []hit `json:"vector"`
		}

		out := output{}
		for _, h := range lexHits {
			preview := ""
			if doc, err := bleveIdx.GetDocument(h.ID); err == nil && doc != nil {
				for _, f := range doc.Fields {
					if f.Name() == "text" {
						val := string(f.Value())
						if len(val) > 80 {
							preview = val[:77] + "..."
						} else {
							preview = val
						}
						break
					}
				}
			}
			out.Lexical = append(out.Lexical, hit{ID: h.ID, Score: float32(h.Score), Text: preview})
		}
		for _, vr := range vecResults {
			fullText := ""
			if doc, err := bleveIdx.GetDocument(vr.ID); err == nil && doc != nil {
				for _, f := range doc.Fields {
					if f.Name() == "text" {
						fullText = string(f.Value())
						break
					}
				}
			}
			out.Vector = append(out.Vector, hit{ID: vr.ID, Score: vr.Score, Text: fullText})
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print detailed version information including build commit and date.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Semango %s\n", version)
		fmt.Printf("  Commit:     %s\n", commit)
		fmt.Printf("  Built:      %s\n", date)
		fmt.Printf("  Go version: %s\n", runtime.Version())
		fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Model Context Protocol (MCP) server commands.",
	Long:  `Start an MCP server to allow LLMs to use 🥭 Semango as a search tool.`,
}

var mcpStdioCmd = &cobra.Command{
	Use:   "stdio",
	Short: "Start MCP server using stdin/stdout transport.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if AppConfig == nil {
			return util.NewError("Configuration not loaded. Pass --config path.")
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			slog.Info("Received shutdown signal, stopping MCP stdio server...")
			cancel()
		}()
		_ = ctx // MCP server will be updated to use ctx in the future

		searcher, err := search.NewSearcher(AppConfig)
		if err != nil {
			return util.WrapError(err, "Failed to initialize searcher for MCP")
		}
		srv := mcp.NewServer(AppConfig, searcher)
		return srv.ServeStdio()
	},
}

var mcpSSECmd = &cobra.Command{
	Use:   "sse",
	Short: "Start MCP server using SSE transport.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if AppConfig == nil {
			return util.NewError("Configuration not loaded. Pass --config path.")
		}
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")
		addr := fmt.Sprintf("%s:%d", host, port)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			slog.Info("Received shutdown signal, stopping MCP SSE server...")
			cancel()
		}()
		_ = ctx // MCP server will be updated to use ctx in the future

		searcher, err := search.NewSearcher(AppConfig)
		if err != nil {
			return util.WrapError(err, "Failed to initialize searcher for MCP")
		}
		srv := mcp.NewServer(AppConfig, searcher)
		return srv.ServeSSE(addr)
	},
}

func init() {
	// Logger is initialized by importing internal/util
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(pdfExtractCmd)
	rootCmd.AddCommand(indexCmd)
	indexCmd.AddCommand(indexStatsCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpStdioCmd)
	mcpCmd.AddCommand(mcpSSECmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cli.NewInstallCmd(version))
	rootCmd.AddCommand(cli.NewModelsCmd())

	mcpSSECmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	mcpSSECmd.Flags().StringP("host", "H", "localhost", "Host to listen on")

	initCmd.Flags().StringP("file", "f", config.DefaultConfigPath, "Path to write the configuration file")
	rootCmd.PersistentFlags().StringP("config", "c", config.DefaultConfigPath, "Path to the configuration file")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")

	pdfExtractCmd.Flags().String("abs", "", "Absolute path to PDF")
}

func Execute() {
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	if err := rootCmd.Execute(); err != nil {
		// Cobra already prints the error, but we log it with our structured format.
		// Check if it's already a SemangoError, if not, wrap it for consistent logging.
		if _, ok := err.(*util.SemangoError); !ok {
			err = util.WrapError(err, "Command execution failed")
		}
		util.LogError(util.Logger, err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}
