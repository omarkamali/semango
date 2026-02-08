package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/ingest"
	"github.com/omarkamali/semango/internal/ingest/tabular"
	"github.com/omarkamali/semango/internal/storage"
	"github.com/omarkamali/semango/pkg/types"
)

// Manager glues: filesystem crawler -> loaders -> embedder -> indexes.
type Manager struct {
	cfg      *config.Config
	embedder ingest.Embedder
	loaders  []ingest.Loader
}

func NewManager(cfg *config.Config, embedder ingest.Embedder) *Manager {
	// register loaders once
	ls := []ingest.Loader{
		ingest.NewTextLoader(cfg.Files.ChunkSize, cfg.Files.ChunkOverlap),
		ingest.NewCodeLoader(false, 5*1024*1024),
		ingest.NewPDFLoader(
			cfg.Files.ChunkSize,
			cfg.Files.ChunkOverlap,
			ingest.WithPDFTimeout(time.Duration(cfg.Files.PDFTimeoutSeconds)*time.Second),
			ingest.WithPDFHeartbeatInterval(time.Duration(cfg.Files.PDFProgressIntervalSeconds)*time.Second),
		),
		&ingest.ImageLoader{},
		tabular.NewCSVLoader(cfg.Tabular),
		tabular.NewJSONLoader(cfg.Tabular),
		tabular.NewParquetLoader(cfg.Tabular),
		tabular.NewSQLiteLoader(cfg.Tabular),
		tabular.NewExcelLoader(cfg.Tabular),
	}
	return &Manager{cfg: cfg, embedder: embedder, loaders: ls}
}

func (m *Manager) loaderForExt(ext string) ingest.Loader {
	for _, l := range m.loaders {
		for _, e := range l.Extensions() {
			if e == ext {
				return l
			}
		}
	}
	return nil
}

// ProcessFile ingests one path (relative & absolute) into vector + lexical indexes.
func (m *Manager) ProcessFile(ctx context.Context, relPath, absPath string) error {
	ext := filepath.Ext(relPath)
	l := m.loaderForExt(ext)
	if l == nil {
		slog.Warn("No suitable loader found for file", "path", relPath, "extension", ext)
		return nil
	}
	reps, err := l.Load(ctx, relPath, absPath)
	if err != nil {
		return err
	}
	if len(reps) == 0 {
		return nil
	}

	// Embed textual reps (only those with Text)
	var texts []string
	var idxMap []int
	for i, r := range reps {
		if r.Text != "" {
			texts = append(texts, r.Text)
			idxMap = append(idxMap, i)
		}
	}
	if len(texts) > 0 {
		vecs, err := m.embedder.Embed(ctx, texts)
		if err != nil {
			return err
		}
		for j, v := range vecs {
			reps[idxMap[j]].Vector = v
		}
	}

	// Open indexes once
	bleveIdx, err := storage.OpenOrCreateBleveIndex(m.cfg.Lexical.IndexPath)
	if err != nil {
		return err
	}
	defer bleveIdx.Close()

	faissPath := filepath.Join(filepath.Dir(m.cfg.Lexical.IndexPath), "faiss.index")
	vecIdx, err := storage.NewFaissVectorIndex(ctx, faissPath, m.embedder.Dimension(), types.MetricInnerProduct)
	if err != nil {
		return err
	}
	defer vecIdx.Close()

	// Index loop
	for _, r := range reps {
		if err := bleveIdx.IndexDocument(r.ID, r.Text, r.Path, r.Meta); err != nil {
			slog.Error("bleve index error", "id", r.ID, "err", err)
		}
		if r.Vector != nil {
			if err := vecIdx.Upsert(ctx, r.ID, r.Vector); err != nil {
				slog.Error("faiss upsert error", "id", r.ID, "err", err)
			}
		}
	}
	slog.Info("Indexed", "file", relPath, "chunks", len(reps))
	return nil
}

// RunIndexing performs a full crawl and indexes all files.
func (m *Manager) RunIndexing(ctx context.Context) (int, error) {
	slog.Info("Starting indexing process...")
	slog.Info("Indexing runtime", "goos", runtime.GOOS, "goarch", runtime.GOARCH)
	rootDir, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("failed to get working directory: %w", err)
	}

	filePathChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go ingest.Crawl(m.cfg.Files, filePathChan, errChan)

	var filesProcessedCount atomic.Int64
	var filesFailedCount atomic.Int64
	var currentFile atomic.Value
	currentFile.Store("")
	start := time.Now()
	stopHeartbeat := make(chan struct{})
	defer close(stopHeartbeat)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				slog.Info("Indexing heartbeat",
					"processed", filesProcessedCount.Load(),
					"failed", filesFailedCount.Load(),
					"elapsed", time.Since(start).String(),
					"current_file", currentFile.Load(),
				)
			case <-stopHeartbeat:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	for relPath := range filePathChan {
		if err := ctx.Err(); err != nil {
			return int(filesProcessedCount.Load()), err
		}
		currentFile.Store(relPath)
		absPath := filepath.Join(rootDir, relPath)
		if err := m.ProcessFile(ctx, relPath, absPath); err != nil {
			slog.Error("Failed to process file", "path", relPath, "error", err)
			filesFailedCount.Add(1)
			continue
		}
		filesProcessedCount.Add(1)
	}

	select {
	case err := <-errChan:
		if err != nil {
			return int(filesProcessedCount.Load()), err
		}
	default:
	}

	slog.Info("Indexing process completed.", "files_processed", filesProcessedCount.Load(), "files_failed", filesFailedCount.Load())
	return int(filesProcessedCount.Load()), nil
}

// RunReconciliation ensures the index is up to date with the filesystem.
func (m *Manager) RunReconciliation(ctx context.Context) error {
	slog.Info("Running index reconciliation...")

	// 1. Incremental indexing (add/update)
	// Currently ProcessFile always re-indexes, which is safe but not most efficient.
	_, err := m.RunIndexing(ctx)
	if err != nil {
		return err
	}

	// 2. Cleanup missing files
	bleveIdx, err := storage.OpenOrCreateBleveIndex(m.cfg.Lexical.IndexPath)
	if err != nil {
		return err
	}
	defer bleveIdx.Close()

	allDocs, err := bleveIdx.GetAllDocs()
	if err != nil {
		return err
	}

	rootDir, _ := os.Getwd()
	deletedCount := 0
	for id, relPath := range allDocs {
		absPath := filepath.Join(rootDir, relPath)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			slog.Info("File no longer exists, removing from index", "path", relPath)
			if err := bleveIdx.Delete(id); err != nil {
				slog.Error("Failed to delete from bleve", "id", id, "err", err)
			}
			deletedCount++
			// Note: We skip FAISS deletion for now as it's not easily supported by go-faiss
		}
	}

	if deletedCount > 0 {
		slog.Info("Cleanup completed", "deleted_docs", deletedCount)
	}

	return nil
}
