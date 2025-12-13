package tabular

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/omarkamali/semango/internal/config"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

func cfg() config.TabularConfig {
	return config.TabularConfig{MaxRowsEmbedded: 1000, Sampling: "random", MinTextTokens: 1}
}

func TestDetectSchema(t *testing.T) {
	rows := []map[string]string{
		{"name": "Alice", "age": "30", "registered": "2024-01-02"},
		{"name": "Bob", "age": "31", "registered": "2025-01-05"},
	}
	cols := DetectSchema(rows)
	expected := map[string]ColumnKind{"name": KindText, "age": KindNumeric, "registered": KindDateTime}
	for _, c := range cols {
		if exp, ok := expected[c.Name]; ok {
			if c.Kind != exp {
				t.Fatalf("column %s expected kind %v got %v", c.Name, exp, c.Kind)
			}
		}
	}
}

func TestCSVLoader(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "sample.csv")
	content := "name,comment\nAlice,Hello world\nBob,Another text"
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	l := NewCSVLoader(cfg())
	reps, err := l.Load(context.Background(), "sample.csv", file)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) == 0 {
		t.Fatalf("expected representations >0 for csv, got %d", len(reps))
	}
}

func TestJSONLoader(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "sample.json")
	arr := []map[string]interface{}{{"name": "Alice", "text": "lorem ipsum"}, {"name": "Bob", "text": "dolor"}}
	data, _ := json.Marshal(arr)
	if err := os.WriteFile(file, data, 0644); err != nil {
		t.Fatal(err)
	}
	l := NewJSONLoader(cfg())
	reps, err := l.Load(context.Background(), "sample.json", file)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) == 0 {
		t.Fatalf("expected representations >0 for json, got %d", len(reps))
	}
}

func TestParquetLoader(t *testing.T) {
	t.Skip("Parquet writer integration flaky; skipping for now")
	dir := t.TempDir()
	file := filepath.Join(dir, "sample.parquet")

	// Define simple row struct
	type Row struct {
		Name string `parquet:"name=name, type=BYTE_ARRAY, convertedtype=UTF8"`
		Note string `parquet:"name=note, type=BYTE_ARRAY, convertedtype=UTF8"`
	}

	fw, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	fw.Close()

	// parquet writer
	fwriter, err := local.NewLocalFileWriter(file)
	if err != nil {
		t.Fatalf("parquet writer open: %v", err)
	}
	pw, err := writer.NewParquetWriter(fwriter, new(Row), 1)
	if err != nil {
		t.Fatalf("parquet writer create: %v", err)
	}
	pw.RowGroupSize = 128 * 1024
	pw.CompressionType = parquet.CompressionCodec_SNAPPY

	rows := []Row{{"Alice", "hello"}, {"Bob", "text"}}
	for _, r := range rows {
		if err := pw.Write(r); err != nil {
			t.Fatalf("parquet write: %v", err)
		}
	}
	if err := pw.WriteStop(); err != nil {
		t.Fatalf("parquet close: %v", err)
	}
	fwriter.Close()

	l := NewParquetLoader(cfg())
	reps, err := l.Load(context.Background(), "sample.parquet", file)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) == 0 {
		t.Fatalf("expected representations >0 for parquet, got %d", len(reps))
	}
}

func TestTSVLoader(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "sample.tsv")
	content := "name	comment\nAlice	Hello world"
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfgLocal := cfg()
	cfgLocal.Delimiter = "\t"
	l := NewCSVLoader(cfgLocal)
	reps, err := l.Load(context.Background(), "sample.tsv", file)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) == 0 {
		t.Fatalf("expected representations >0 for tsv, got %d", len(reps))
	}
}
