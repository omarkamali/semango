package tabular

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"

	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/ingest"
)

// SQLiteLoader treats each table as a separate dataset similar to a file.
// Supported extensions: .sqlite .db .sqlite3.
type SQLiteLoader struct {
	cfg config.TabularConfig
}

func NewSQLiteLoader(cfg config.TabularConfig) *SQLiteLoader { return &SQLiteLoader{cfg: cfg} }

func (l *SQLiteLoader) Extensions() []string { return []string{".sqlite", ".db", ".sqlite3"} }

func (l *SQLiteLoader) Load(ctx context.Context, relPath string, absPath string) ([]ingest.Representation, error) {
	slog.Info("Loading SQLite DB", "relPath", relPath)

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout=5000", absPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rowsTbl, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	defer rowsTbl.Close()

	var allReps []ingest.Representation

	for rowsTbl.Next() {
		var table string
		if err := rowsTbl.Scan(&table); err != nil {
			continue
		}

		query := fmt.Sprintf("SELECT * FROM %s", table)
		r, err := db.QueryContext(ctx, query)
		if err != nil {
			continue
		}

		cols, _ := r.Columns()
		// iterate rows up to cap
		var maps []map[string]string
		capRows := l.cfg.MaxRowsEmbedded * 2
		count := 0
		for r.Next() {
			vals := make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := r.Scan(ptrs...); err != nil {
				continue
			}
			m := map[string]string{}
			for i, c := range cols {
				m[c] = fmt.Sprintf("%v", vals[i])
			}
			maps = append(maps, m)
			count++
			if count >= capRows {
				break
			}
		}
		r.Close()

		reps, _ := BuildRepresentations(maps, relPath+"#"+table, l.cfg)
		for i := range reps {
			if reps[i].Meta == nil {
				reps[i].Meta = map[string]string{}
			}
			reps[i].Meta["table"] = table
		}
		allReps = append(allReps, reps...)
	}

	return allReps, nil
}
