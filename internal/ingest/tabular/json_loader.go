package tabular

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/ingest"
)

// JSONLoader handles .json (array) and .jsonl files containing objects.
type JSONLoader struct {
	cfg config.TabularConfig
}

func NewJSONLoader(cfg config.TabularConfig) *JSONLoader { return &JSONLoader{cfg: cfg} }

func (l *JSONLoader) Extensions() []string { return []string{".json", ".jsonl"} }

func (l *JSONLoader) Load(ctx context.Context, relPath string, absPath string) ([]ingest.Representation, error) {
	slog.Info("Loading JSON file", "relPath", relPath)

	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rows []map[string]string

	ext := filepath.Ext(absPath)
	if ext == ".jsonl" {
		// Each line is a JSON object
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Bytes()
			var obj map[string]interface{}
			if err := json.Unmarshal(line, &obj); err != nil {
				slog.Warn("Skipping invalid JSONL line", "path", relPath, "err", err)
				continue
			}
			rows = append(rows, stringifyMap(obj))
			if len(rows) >= l.cfg.MaxRowsEmbedded*2 {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	} else {
		// Could be an array or single object – stream decode.
		dec := json.NewDecoder(f)
		// Peek first token
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if delim, ok := tok.(json.Delim); ok && delim == '[' {
			// Array start
			for dec.More() {
				var obj map[string]interface{}
				if err := dec.Decode(&obj); err != nil {
					return nil, err
				}
				rows = append(rows, stringifyMap(obj))
				if len(rows) >= l.cfg.MaxRowsEmbedded*2 {
					break
				}
			}
			// Consume closing ]
			_, _ = dec.Token()
		} else {
			// Single object (tok is already part of object?), rewind impossible; reread entire file
			f.Seek(0, io.SeekStart)
			var obj map[string]interface{}
			if err := json.NewDecoder(f).Decode(&obj); err != nil {
				return nil, err
			}
			rows = append(rows, stringifyMap(obj))
		}
	}

	return BuildRepresentations(rows, relPath, l.cfg)
}

func stringifyMap(in map[string]interface{}) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		switch vv := v.(type) {
		case string:
			out[k] = vv
		default:
			b, _ := json.Marshal(v)
			out[k] = string(b)
		}
	}
	return out
}
