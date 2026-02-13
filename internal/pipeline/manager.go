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
	fpStore  *FingerprintStore

	// Shared indexing status – safe for concurrent reads from the API.
	status IndexingStatus
}

// IndexingStatus tracks live indexing progress. Fields are read/written via
// atomic operations so they are safe for concurrent access from the HTTP handler.
type IndexingStatus struct {
	Active      atomic.Bool
	StartedAt   atomic.Int64 // UnixNano
	FilesTotal  atomic.Int64
	FilesQueued atomic.Int64
	FilesDone   atomic.Int64
	FilesFailed atomic.Int64
	CurrentFile atomic.Value // string
}

// IndexingStatusSnapshot is a JSON-friendly snapshot of IndexingStatus.
type IndexingStatusSnapshot struct {
	Active      bool    `json:"active"`
	ElapsedMs   int64   `json:"elapsed_ms,omitempty"`
	FilesTotal  int64   `json:"files_total"`
	FilesQueued int64   `json:"files_queued"`
	FilesDone   int64   `json:"files_done"`
	FilesFailed int64   `json:"files_failed"`
	CurrentFile string  `json:"current_file,omitempty"`
	EtaMs       int64   `json:"eta_ms,omitempty"`
	Progress    float64 `json:"progress"` // 0.0 – 1.0
}

// Snapshot returns a point-in-time copy suitable for JSON serialisation.
func (s *IndexingStatus) Snapshot() IndexingStatusSnapshot {
	active := s.Active.Load()
	started := s.StartedAt.Load()
	queued := s.FilesQueued.Load()
	done := s.FilesDone.Load()
	failed := s.FilesFailed.Load()
	total := s.FilesTotal.Load()

	snap := IndexingStatusSnapshot{
		Active:      active,
		FilesTotal:  total,
		FilesQueued: queued,
		FilesDone:   done,
		FilesFailed: failed,
	}

	if cur, ok := s.CurrentFile.Load().(string); ok {
		snap.CurrentFile = cur
	}

	if active && started > 0 {
		elapsed := time.Since(time.Unix(0, started))
		snap.ElapsedMs = elapsed.Milliseconds()

		processed := done + failed
		if processed > 0 && queued > 0 {
			avgPerFile := elapsed / time.Duration(processed)
			remaining := queued - processed
			if remaining > 0 {
				snap.EtaMs = (avgPerFile * time.Duration(remaining)).Milliseconds()
			}
		}
	}

	if total > 0 {
		snap.Progress = float64(done) / float64(total)
	}

	return snap
}

// Status returns the shared indexing status so the API server can read it.
func (m *Manager) Status() *IndexingStatus { return &m.status }

func NewManager(cfg *config.Config, embedder ingest.Embedder) *Manager {
	pdfTimeout := time.Duration(cfg.Files.PDFTimeoutSeconds) * time.Second
	if pdfTimeout == 0 {
		pdfTimeout = 15 * time.Minute
	}
	pdfHeartbeat := time.Duration(cfg.Files.PDFProgressIntervalSeconds) * time.Second
	if pdfHeartbeat == 0 {
		pdfHeartbeat = 30 * time.Second
	}

	// Fingerprint store lives next to the lexical index.
	fpPath := filepath.Join(filepath.Dir(cfg.Lexical.IndexPath), "file_fingerprints.json")

	ls := []ingest.Loader{
		ingest.NewTextLoader(cfg.Files.ChunkSize, cfg.Files.ChunkOverlap),
		ingest.NewCodeLoader(false, 5*1024*1024),
		ingest.NewPDFLoader(
			cfg.Files.ChunkSize,
			cfg.Files.ChunkOverlap,
			ingest.WithPDFTimeout(pdfTimeout),
			ingest.WithPDFHeartbeatInterval(pdfHeartbeat),
		),
		&ingest.ImageLoader{},
		tabular.NewCSVLoader(cfg.Tabular),
		tabular.NewJSONLoader(cfg.Tabular),
		tabular.NewParquetLoader(cfg.Tabular),
		tabular.NewSQLiteLoader(cfg.Tabular),
		tabular.NewExcelLoader(cfg.Tabular),
	}
	return &Manager{cfg: cfg, embedder: embedder, loaders: ls, fpStore: NewFingerprintStore(fpPath)}
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
	vecIdx, vecErr := storage.NewFaissVectorIndex(ctx, faissPath, m.embedder.Dimension(), types.MetricInnerProduct)
	if vecErr != nil {
		slog.Warn("Vector index unavailable, indexing lexical only", "error", vecErr)
	} else {
		defer vecIdx.Close()
	}

	// Index loop
	for _, r := range reps {
		if err := bleveIdx.IndexDocument(r.ID, r.Text, r.Path, r.Meta); err != nil {
			slog.Error("bleve index error", "id", r.ID, "err", err)
		}
		if r.Vector != nil && vecIdx != nil {
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

	go ingest.Crawl(ctx, m.cfg.Files, filePathChan, errChan)

	// Reset shared status.
	m.status.Active.Store(true)
	m.status.StartedAt.Store(time.Now().UnixNano())
	m.status.FilesTotal.Store(0)
	m.status.FilesQueued.Store(0)
	m.status.FilesDone.Store(0)
	m.status.FilesFailed.Store(0)
	m.status.CurrentFile.Store("")
	defer m.status.Active.Store(false)

	start := time.Now()
	stopHeartbeat := make(chan struct{})
	defer close(stopHeartbeat)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cur, _ := m.status.CurrentFile.Load().(string)
				slog.Info("Indexing heartbeat",
					"queued", m.status.FilesQueued.Load(),
					"processed", m.status.FilesDone.Load(),
					"failed", m.status.FilesFailed.Load(),
					"elapsed", time.Since(start).String(),
					"current_file", cur,
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
			return int(m.status.FilesDone.Load()), err
		}
		absPath := filepath.Join(rootDir, relPath)

		m.status.FilesTotal.Add(1)

		// Skip files whose mtime+size haven't changed since last index.
		if !m.fpStore.Changed(relPath, absPath) {
			slog.Debug("Skipping unchanged file", "path", relPath)
			m.status.FilesDone.Add(1)
			continue
		}

		m.status.FilesQueued.Add(1)
		m.status.CurrentFile.Store(relPath)
		if err := m.ProcessFile(ctx, relPath, absPath); err != nil {
			slog.Error("Failed to process file", "path", relPath, "error", err)
			m.status.FilesFailed.Add(1)
			continue
		}
		m.fpStore.Record(relPath, absPath)
		m.status.FilesDone.Add(1)
	}

	if err := m.fpStore.Save(); err != nil {
		slog.Error("Failed to persist fingerprint store", "err", err)
	}

	select {
	case err := <-errChan:
		if err != nil {
			return int(m.status.FilesDone.Load()), err
		}
	default:
	}

	slog.Info("Indexing process completed.",
		"files_total", m.status.FilesTotal.Load(),
		"files_processed", m.status.FilesDone.Load(),
		"files_failed", m.status.FilesFailed.Load())
	return int(m.status.FilesDone.Load()), nil
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
			m.fpStore.Remove(relPath)
			deletedCount++
			// Note: We skip FAISS deletion for now as it's not easily supported by go-faiss
		}
	}

	if deletedCount > 0 {
		_ = m.fpStore.Save()
		slog.Info("Cleanup completed", "deleted_docs", deletedCount)
	}

	return nil
}
