package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/omarkamali/semango/internal/config"
)

// JSONLoader supports both .json (array) and .jsonl (one object per line).
type JSONLoader struct {
	cfg *config.TabularConfig
}

func NewJSONLoader(cfg *config.TabularConfig) *JSONLoader {
	return &JSONLoader{cfg: cfg}
}

func (jl *JSONLoader) Extensions() []string {
	return []string{".json", ".jsonl"}
}

func (jl *JSONLoader) Load(ctx context.Context, relPath string, absPath string) ([]Representation, error) {
	slog.Info("Loading JSON file", "path", relPath)

	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reps []Representation
	maxRows := 50000
	if jl.cfg != nil && jl.cfg.MaxRowsEmbedded > 0 {
		maxRows = jl.cfg.MaxRowsEmbedded
	}

	// Heuristic: If file extension ends with .jsonl treat as line delimited
	if strings.HasSuffix(strings.ToLower(absPath), "jsonl") {
		scanner := bufio.NewScanner(f)
		rowIdx := 0
		for scanner.Scan() {
			if rowIdx >= maxRows {
				break
			}
			var obj map[string]interface{}
			if err := json.Unmarshal(scanner.Bytes(), &obj); err != nil {
				continue // skip malformed lines
			}
			row := convertToStringMap(obj)
			if rep, ok := BuildRepresentationForRow(relPath, rowIdx, row, jl.cfg); ok {
				reps = append(reps, rep)
			}
			rowIdx++
		}
		return reps, nil
	}

	// Otherwise treat as regular JSON array or single object
	var data interface{}
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}

	switch v := data.(type) {
	case []interface{}:
		for rowIdx, item := range v {
			if rowIdx >= maxRows {
				break
			}
			if m, ok := item.(map[string]interface{}); ok {
				row := convertToStringMap(m)
				if rep, ok := BuildRepresentationForRow(relPath, rowIdx, row, jl.cfg); ok {
					reps = append(reps, rep)
				}
			}
		}
	case map[string]interface{}:
		row := convertToStringMap(v)
		if rep, ok := BuildRepresentationForRow(relPath, 0, row, jl.cfg); ok {
			reps = append(reps, rep)
		}
	default:
		// unsupported
	}

	return reps, nil
}

func convertToStringMap(in map[string]interface{}) map[string]string {
	out := make(map[string]string)
	for k, v := range in {
		out[k] = stringifyJSONValue(v)
	}
	return out
}

func stringifyJSONValue(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case float64, int, int64, bool:
		return fmt.Sprintf("%v", t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
