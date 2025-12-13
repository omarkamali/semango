package ingest

import (
	"context"
	"encoding/csv"
	"log/slog"
	"os"

	"github.com/omarkamali/semango/internal/config"
)

// CSVLoader loads .csv files and converts each row into a Representation.
type CSVLoader struct {
	cfg *config.TabularConfig
}

func NewCSVLoader(cfg *config.TabularConfig) *CSVLoader {
	return &CSVLoader{cfg: cfg}
}

func (cl *CSVLoader) Extensions() []string {
	return []string{".csv"}
}

func (cl *CSVLoader) Load(ctx context.Context, relPath string, absPath string) ([]Representation, error) {
	slog.Info("Loading CSV file", "path", relPath)

	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	// TODO: make delimiter configurable if needed.
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	var reps []Representation
	rowIdx := 0
	maxRows := 50000
	if cl.cfg != nil && cl.cfg.MaxRowsEmbedded > 0 {
		maxRows = cl.cfg.MaxRowsEmbedded
	}

	for {
		if rowIdx >= maxRows {
			break
		}
		record, err := reader.Read()
		if err != nil {
			// EOF is expected
			break
		}
		row := make(map[string]string)
		for i, col := range header {
			if i < len(record) {
				row[col] = record[i]
			}
		}
		if rep, ok := BuildRepresentationForRow(relPath, rowIdx, row, cl.cfg); ok {
			reps = append(reps, rep)
		}
		rowIdx++
	}

	slog.Debug("CSV loader created reps", "count", len(reps))
	return reps, nil
}
