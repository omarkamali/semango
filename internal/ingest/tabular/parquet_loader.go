package tabular

import (
	"context"
	"log/slog"

	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/ingest"

	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/reader"
)

// ParquetLoader streams rows from .parquet files and converts to representations.

type ParquetLoader struct {
	cfg config.TabularConfig
}

func NewParquetLoader(cfg config.TabularConfig) *ParquetLoader { return &ParquetLoader{cfg: cfg} }

func (l *ParquetLoader) Extensions() []string { return []string{".parquet"} }

func (l *ParquetLoader) Load(ctx context.Context, relPath string, absPath string) ([]ingest.Representation, error) {
	slog.Info("Loading Parquet file", "relPath", relPath)

	fr, err := local.NewLocalFileReader(absPath)
	if err != nil {
		return nil, err
	}
	defer fr.Close()

	// read rows as generic map[string]interface{}
	pr, err := reader.NewParquetReader(fr, map[string]interface{}{}, 1)
	if err != nil {
		return nil, err
	}
	defer pr.ReadStop()

	num := int(pr.GetNumRows())
	rowsCap := l.cfg.MaxRowsEmbedded * 2
	if rowsCap <= 0 {
		rowsCap = 50000
	}

	rowsToRead := num
	if rowsToRead == 0 || rowsToRead > rowsCap {
		rowsToRead = rowsCap
	}

	// Read into generic interface map[string]interface{}
	var rows []map[string]string
	batchSize := 1000
	for read := 0; read < rowsToRead; {
		n := batchSize
		if rowsToRead-read < n {
			n = rowsToRead - read
		}
		data := make([]interface{}, n)
		if err := pr.Read(&data); err != nil {
			return nil, err
		}
		for _, rowData := range data {
			if rowData == nil {
				continue
			}
			if m, ok := rowData.(map[string]interface{}); ok {
				rows = append(rows, stringifyMap(m))
			}
		}
		read += n
	}

	return BuildRepresentations(rows, relPath, l.cfg)
}
