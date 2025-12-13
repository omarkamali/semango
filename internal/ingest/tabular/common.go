package tabular

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/ingest"
)

// ColumnKind represents the detected semantic type of a table column.
// We only distinguish kinds that influence embedding / indexing logic.
//go:generate stringer -type=ColumnKind
// (not critical – omit actual generation to keep build simple)

type ColumnKind int

const (
	KindUnknown     ColumnKind = iota
	KindText                   // long or free-form string
	KindCategorical            // finite small vocabulary of strings
	KindNumeric                // any int / float
	KindDateTime               // ISO / RFC3339 determinable timestamp
	KindBinary                 // base64 / raw bytes, ignored
)

// Column contains name & inferred kind.
type Column struct {
	Name string
	Kind ColumnKind
}

// DetectSchema inspects the first N rows and returns a slice of Column with the best-guess kinds.
func DetectSchema(rows []map[string]string) []Column {
	if len(rows) == 0 {
		return nil
	}
	// Build per-column counters.
	type stats struct {
		numeric, datetime, total int
		uniques                  map[string]bool
		longestToken             int
	}
	colStats := map[string]*stats{}
	for _, row := range rows {
		for k, v := range row {
			st, ok := colStats[k]
			if !ok {
				st = &stats{uniques: map[string]bool{}}
				colStats[k] = st
			}
			st.total++
			vv := strings.TrimSpace(v)
			if vv == "" {
				continue
			}
			if isNumeric(vv) {
				st.numeric++
			} else if isDateTime(vv) {
				st.datetime++
			}
			st.uniques[vv] = true
			if len(vv) > st.longestToken {
				st.longestToken = len(vv)
			}
		}
	}
	// decide kinds
	var cols []Column
	for name, st := range colStats {
		kind := KindText              // default
		if st.numeric*2 >= st.total { // majority numeric
			kind = KindNumeric
		} else if st.datetime*2 >= st.total {
			kind = KindDateTime
		} else {
			// Consider categorical
			uniqueRatio := float64(len(st.uniques)) / float64(st.total)
			if uniqueRatio < 0.02 && st.longestToken < 40 {
				kind = KindCategorical
			} else if st.longestToken == 0 {
				kind = KindUnknown
			}
		}
		cols = append(cols, Column{Name: name, Kind: kind})
	}
	// stable order
	sort.Slice(cols, func(i, j int) bool { return cols[i].Name < cols[j].Name })
	return cols
}

var reNumeric = regexp.MustCompile(`^[-+]?(?:\d+\.?\d*|\.\d+)$`)

func isNumeric(s string) bool {
	return reNumeric.MatchString(s)
}

var reDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`) // very naive YYYY-MM-DD

func isDateTime(s string) bool {
	// cheap check – we do not parse full RFCs here
	return reDate.MatchString(s)
}

// BuildRepresentations converts table rows into ingest.Representation slices according
// to config rules (sampling, token thresholds, etc.).
// rows parameter MAY be truncated already (e.g. sampling pre-applied by loader).
func BuildRepresentations(rows []map[string]string, relPath string, cfg config.TabularConfig) ([]ingest.Representation, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	schema := DetectSchema(rows)

	// compute schema hash – deterministic: join name+kind sorted
	var sb strings.Builder
	for _, c := range schema {
		sb.WriteString(c.Name)
		sb.WriteRune(':')
		sb.WriteString(strconv.Itoa(int(c.Kind)))
		sb.WriteRune(',')
	}
	schemaHash := sha256.Sum256([]byte(sb.String()))
	schemaHashHex := hex.EncodeToString(schemaHash[:6]) // first 6 bytes good enough

	numRows := len(rows)

	var reps []ingest.Representation

	// helper to emit per-row rep
	emitRow := func(i int, row map[string]string) {
		// Build joined text from Text & Categorical columns.
		var textParts []string
		for _, col := range schema {
			val := strings.TrimSpace(row[col.Name])
			if val == "" {
				continue
			}
			switch col.Kind {
			case KindText, KindCategorical:
				textParts = append(textParts, col.Name+": "+val)
			}
		}
		joinedText := strings.Join(textParts, "\n")
		// token count threshold
		if joinedText == "" || len(strings.Split(joinedText, " ")) < cfg.MinTextTokens {
			return
		}
		rep := ingest.Representation{
			ID:       ingest.ChunkID(relPath, "table_row", int64(i)),
			Path:     relPath,
			Modality: "table_row",
			Text:     joinedText,
			Meta: map[string]string{
				"row":      strconv.Itoa(i),
				"schema":   schemaHashHex,
				"num_rows": strconv.Itoa(numRows),
				"source":   "TabularLoader",
				"path":     relPath,
			},
		}
		// add each column raw value to meta (flattened)
		for k, v := range row {
			rep.Meta["col."+k] = v
		}
		reps = append(reps, rep)
	}

	// sampling logic
	if numRows > cfg.MaxRowsEmbedded {
		step := float64(numRows) / float64(cfg.MaxRowsEmbedded)
		switch cfg.Sampling {
		case "stratified":
			// simple proportional step sampling – guarantee first row, last row
			idx := 0.0
			for i := 0; i < numRows && len(reps) < cfg.MaxRowsEmbedded; i++ {
				if float64(i) >= idx {
					emitRow(i, rows[i])
					idx += step
				}
			}
		default: // random
			// reservoir sampling first MaxRowsEmbedded rows
			importMathRandOnce()
			selected := randPerm(numRows)[:cfg.MaxRowsEmbedded]
			sort.Ints(selected)
			for _, i := range selected {
				emitRow(i, rows[i])
			}
		}
	} else {
		for i, row := range rows {
			emitRow(i, row)
		}
	}

	// file-level summary representation
	summaryText := buildSummaryText(relPath, numRows, schema)
	reps = append(reps, ingest.Representation{
		ID:       ingest.ChunkID(relPath, "table_file_summary", 0),
		Path:     relPath,
		Modality: "text",
		Text:     summaryText,
		Meta: map[string]string{
			"source": "TabularLoader",
			"kind":   "file_summary",
			"schema": schemaHashHex,
			"path":   relPath,
		},
	})

	// schema representation
	schemaText := buildSchemaText(schema)
	reps = append(reps, ingest.Representation{
		ID:       ingest.ChunkID(relPath, "table_schema", 0),
		Path:     relPath,
		Modality: "text",
		Text:     schemaText,
		Meta: map[string]string{
			"source": "TabularLoader",
			"kind":   "schema",
			"schema": schemaHashHex,
			"path":   relPath,
		},
	})

	return reps, nil
}

func buildSummaryText(relPath string, numRows int, schema []Column) string {
	var cols []string
	for _, c := range schema {
		cols = append(cols, c.Name)
	}
	return "Tabular file " + relPath + " with " + strconv.Itoa(numRows) + " rows. Columns: " + strings.Join(cols, ", ")
}

func buildSchemaText(schema []Column) string {
	var parts []string
	for _, c := range schema {
		parts = append(parts, c.Name+"("+columnKindString(c.Kind)+")")
	}
	return "Schema: " + strings.Join(parts, ", ")
}

func columnKindString(k ColumnKind) string {
	switch k {
	case KindText:
		return "text"
	case KindNumeric:
		return "numeric"
	case KindDateTime:
		return "datetime"
	case KindCategorical:
		return "categorical"
	default:
		return "unknown"
	}
}

// rand helpers – defer math/rand global usage until needed to keep deterministic tests unless called.

func importMathRandOnce() {
	if rngInitDone {
		return
	}
	rngInitOnce.Do(func() {
		rand.Seed(time.Now().UnixNano())
		rngInitDone = true
	})
}

var (
	rngInitOnce sync.Once
	rngInitDone bool
)

func randPerm(n int) []int {
	importMathRandOnce()
	return rand.Perm(n)
}
