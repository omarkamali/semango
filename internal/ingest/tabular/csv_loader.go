package tabular

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/ingest"
)

// CSVLoader implements ingest.Loader for .csv files.
// It streams rows using encoding/csv and converts them via BuildRepresentations.

type CSVLoader struct {
	cfg       config.TabularConfig
	delimiter rune
}

func NewCSVLoader(cfg config.TabularConfig) *CSVLoader {
	d := ','
	if cfg.Delimiter != "" {
		if cfg.Delimiter == "\t" {
			d = '\t'
		} else {
			d = rune(cfg.Delimiter[0])
		}
	}
	return &CSVLoader{cfg: cfg, delimiter: d}
}

func (l *CSVLoader) Extensions() []string { return []string{".csv", ".tsv"} }

func (l *CSVLoader) Load(ctx context.Context, relPath string, absPath string) ([]ingest.Representation, error) {
	slog.Info("Loading CSV file", "relPath", relPath)
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = l.delimiter
	r.ReuseRecord = true
	headers, err := r.Read()
	if err != nil {
		return nil, err
	}

	var rows []map[string]string
	maxRows := l.cfg.MaxRowsEmbedded * 2 // read extra so sampling has enough, but safeguard memory

	for i := 0; ; i++ {
		record, err := r.Read()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil, err
			}
			if errors.Is(err, io.EOF) {
				break
			}
			// non-EOF error
			slog.Warn("CSV read error", "path", relPath, "err", err)
			break
		}
		row := make(map[string]string, len(headers))
		for idx, h := range headers {
			if idx < len(record) {
				row[h] = record[idx]
			}
		}
		rows = append(rows, row)
		if len(rows) >= maxRows {
			break // don't keep reading huge files – later sampling or embed cap will handle
		}
	}

	return BuildRepresentations(rows, relPath, l.cfg)
}
