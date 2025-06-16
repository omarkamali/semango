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
	"syscall"

	"github.com/blevesearch/go-faiss"
	"github.com/omneity-labs/semango/internal/api"
	"github.com/omneity-labs/semango/internal/config"
	"github.com/omneity-labs/semango/internal/ingest"
	"github.com/omneity-labs/semango/internal/search"
	"github.com/omneity-labs/semango/internal/storage"
	"github.com/omneity-labs/semango/internal/util" // Import the logger package
	"github.com/spf13/cobra"
)

var AppConfig *config.Config // Global config instance

var rootCmd = &cobra.Command{
	Use:   "semango",
	Short: "Semango is a semantic search engine.",
	Long:  `A fast and flexible semantic search engine capable of indexing and searching various file types.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		_ = util.Logger                                                                     // Ensure logger is initialized
		if cmd.Name() == "init" || (cmd.Parent() != nil && cmd.Parent().Name() == "init") { // also skip for subcommands of init if any
			slog.Debug("Skipping configuration loading for init command or its subcommands")
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
		slog.Info("Welcome to Semango! Use -h or --help for available commands.")
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
			// This is a programming error or an issue with command setup, should not happen if PersistentPreRunE works.
			// Using NewError as there's no underlying specific Go error to wrap here.
			cfgErr := util.NewError("Configuration not loaded before index command")
			util.LogError(util.Logger, cfgErr)
			return cfgErr
		}
		slog.Info("Starting indexing process...", "files_config", AppConfig.Files)

		rootDir, err := os.Getwd()
		if err != nil {
			wrappedErr := util.WrapError(err, "Failed to get working directory for indexing")
			util.LogError(util.Logger, wrappedErr)
			return wrappedErr
		}

		filePathChan := make(chan string, 100)
		errChan := make(chan error, 1)

		// Initialize the in-memory store
		repStore := storage.NewInMemoryStore()

		// Open or create Bleve full-text index
		bleveIdx, err := storage.OpenOrCreateBleveIndex(AppConfig.Lexical.IndexPath)
		if err != nil {
			wrappedErr := util.WrapError(err, "Failed to open/create Bleve index", slog.String("path", AppConfig.Lexical.IndexPath))
			util.LogError(util.Logger, wrappedErr)
			return wrappedErr
		}
		defer bleveIdx.Close()

		// Initialize embedder with proper validation
		var embedder ingest.Embedder
		{
			prov := AppConfig.Embedding.Provider
			switch prov {
			case "openai", "": // default to openai
				apiKey := os.Getenv("OPENAI_API_KEY")
				if apiKey == "" {
					return util.NewError("OpenAI API key is required but not found in OPENAI_API_KEY environment variable")
				}
				openCfg := ingest.OpenAIConfig{
					APIKey:     apiKey,
					Model:      AppConfig.Embedding.Model,
					BatchSize:  AppConfig.Embedding.BatchSize,
					Concurrent: AppConfig.Embedding.Concurrent,
				}
				e, err := ingest.NewOpenAIEmbedder(openCfg)
				if err != nil {
					return util.WrapError(err, "Failed to create OpenAI embedder")
				}
				embedder = e
			case "local":
				if AppConfig.Embedding.LocalModelPath == "" {
					return util.NewError("Local model path is required for local embedder provider")
				}
				localCfg := ingest.LocalEmbedderConfig{
					ModelPath: AppConfig.Embedding.LocalModelPath,
					CacheDir:  AppConfig.Embedding.ModelCacheDir,
					BatchSize: AppConfig.Embedding.BatchSize,
					MaxLength: 512, // Default max length
				}
				// Validate configuration
				if err := ingest.ValidateModelConfig(localCfg); err != nil {
					return util.WrapError(err, "Invalid local embedder configuration")
				}
				e, err := ingest.NewLocalEmbedder(localCfg)
				if err != nil {
					return util.WrapError(err, "Failed to create local embedder")
				}
				embedder = e
			default:
				return util.NewError(fmt.Sprintf("Unsupported embedder provider: %s. Supported providers: openai, local", prov))
			}
		}

		// Open or create FAISS vector index
		faissPath := filepath.Join("semango", "index", "faiss.index")
		vecIdx, err := storage.NewFaissVectorIndex(context.Background(), faissPath, embedder.Dimension(), faiss.MetricInnerProduct)
		if err != nil {
			wrappedErr := util.WrapError(err, "Failed to open/create FAISS index", slog.String("path", faissPath))
			util.LogError(util.Logger, wrappedErr)
			return wrappedErr
		}
		defer vecIdx.Close()

		go ingest.Crawl(AppConfig.Files, filePathChan, errChan)

		var filesProcessedCount int
		// totalRepresentations will now be derived from the store

		textLoader := ingest.NewTextLoader(AppConfig.Files.ChunkSize, AppConfig.Files.ChunkOverlap)

		for relPath := range filePathChan {
			absPath := filepath.Join(rootDir, relPath)
			slog.Info("Processing file", "relative_path", relPath, "absolute_path", absPath)

			ext := filepath.Ext(relPath)
			var loader ingest.Loader
			if stringInSlice(ext, textLoader.Extensions()) {
				loader = textLoader
			} else {
				slog.Warn("No suitable loader found for file", "path", relPath, "extension", ext)
				continue
			}

			ctx := context.Background()
			representations, err := loader.Load(ctx, relPath, absPath)
			if err != nil {
				// Log and continue to next file
				util.LogError(util.Logger, util.WrapError(err, "Failed to load file", slog.String("relative_path", relPath), slog.String("absolute_path", absPath)))
				continue
			}

			for _, rep := range representations {
				if err := repStore.Add(rep); err != nil {
					// Log and continue
					util.LogError(util.Logger, util.WrapError(err, "Failed to add representation to store", slog.String("id", rep.ID), slog.String("path", rep.Path)))
					continue
				}
				slog.Info("Generated and stored representation", "id", rep.ID, "path", rep.Path, "modality", rep.Modality, "text_preview", truncateString(rep.Text, 50))

				// Index in Bleve if modality is text
				if rep.Modality == "text" {
					if err := bleveIdx.IndexDocument(rep.ID, rep.Text, rep.Meta); err != nil {
						// Log and continue
						util.LogError(util.Logger, util.WrapError(err, "Failed to index document in Bleve", slog.String("id", rep.ID)))
					}
				}

				// Vector embedding and upsert
				vector, err := embedder.Embed(ctx, []string{rep.Text})
				if err != nil {
					util.LogError(util.Logger, util.WrapError(err, "Embedding failed", slog.String("id", rep.ID)))
				} else if len(vector) == 1 {
					if err := vecIdx.Upsert(ctx, rep.ID, vector[0]); err != nil {
						util.LogError(util.Logger, util.WrapError(err, "Vector upsert failed", slog.String("id", rep.ID)))
					}
				}
			}
			filesProcessedCount++

			// Metrics: increment files_processed counter
			util.DefaultMetrics.IncCounter("files_processed", map[string]string{"ext": ext})
		}

		slog.Debug("filePathChan closed.", "files_crawled_count", filesProcessedCount)

		var crawlerError error
		select {
		case err := <-errChan:
			if err != nil {
				crawlerError = err
			}
		default:
		}

		if crawlerError != nil {
			finalErr := util.WrapError(crawlerError, "Indexing failed due to crawler error")
			util.LogError(util.Logger, finalErr)
			return finalErr
		}

		slog.Info("Indexing process completed.", "files_processed_by_loader", filesProcessedCount, "total_representations_in_store", repStore.Count())
		// Optionally, list all stored representations for debugging
		// for _, rep := range repStore.GetAll() {
		// 	slog.Debug("Stored item", "id", rep.ID, "path", rep.Path)
		// }
		return nil
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
		faissPath := filepath.Join("semango", "index", "faiss.index")
		// Initialize embedder (same logic as indexCmd)
		var embedder ingest.Embedder
		{
			prov := AppConfig.Embedding.Provider
			switch prov {
			case "openai", "": // default to openai
				apiKey := os.Getenv("OPENAI_API_KEY")
				if apiKey == "" {
					return util.NewError("OpenAI API key is required but not found in OPENAI_API_KEY environment variable")
				}
				openCfg := ingest.OpenAIConfig{
					APIKey:     apiKey,
					Model:      AppConfig.Embedding.Model,
					BatchSize:  AppConfig.Embedding.BatchSize,
					Concurrent: AppConfig.Embedding.Concurrent,
				}
				e, err := ingest.NewOpenAIEmbedder(openCfg)
				if err != nil {
					return util.WrapError(err, "Failed to create OpenAI embedder")
				}
				embedder = e
			case "local":
				if AppConfig.Embedding.LocalModelPath == "" {
					return util.NewError("Local model path is required for local embedder provider")
				}
				localCfg := ingest.LocalEmbedderConfig{
					ModelPath: AppConfig.Embedding.LocalModelPath,
					CacheDir:  AppConfig.Embedding.ModelCacheDir,
					BatchSize: AppConfig.Embedding.BatchSize,
					MaxLength: 512, // Default max length
				}
				// Validate configuration
				if err := ingest.ValidateModelConfig(localCfg); err != nil {
					return util.WrapError(err, "Invalid local embedder configuration")
				}
				e, err := ingest.NewLocalEmbedder(localCfg)
				if err != nil {
					return util.WrapError(err, "Failed to create local embedder")
				}
				embedder = e
			default:
				return util.NewError(fmt.Sprintf("Unsupported embedder provider: %s. Supported providers: openai, local", prov))
			}
		}

		queryVecs, err := embedder.Embed(context.Background(), []string{query})
		if err != nil {
			return util.WrapError(err, "Embedding query failed")
		}
		vecIdx, err := storage.NewFaissVectorIndex(context.Background(), faissPath, embedder.Dimension(), faiss.MetricInnerProduct)
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

// Helper function to check if a string is in a slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// Helper function to truncate strings for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	// Logger is initialized by importing internal/util
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(indexCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(serverCmd)
	initCmd.Flags().StringP("file", "f", config.DefaultConfigPath, "Path to write the configuration file")
	rootCmd.PersistentFlags().StringP("config", "c", config.DefaultConfigPath, "Path to the configuration file")
}

func Execute() {
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
