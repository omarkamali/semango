package pipeline

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/blevesearch/go-faiss"
	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/ingest"
	"github.com/omarkamali/semango/internal/ingest/tabular"
	"github.com/omarkamali/semango/internal/storage"
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
		&ingest.PDFLoader{}, &ingest.ImageLoader{},
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

	faissPath := filepath.Join("semango", "index", "faiss.index")
	vecIdx, err := storage.NewFaissVectorIndex(ctx, faissPath, m.embedder.Dimension(), faiss.MetricInnerProduct)
	if err != nil {
		return err
	}
	defer vecIdx.Close()

	// Index loop
	for _, r := range reps {
		if err := bleveIdx.IndexDocument(r.ID, r.Text, r.Meta); err != nil {
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
