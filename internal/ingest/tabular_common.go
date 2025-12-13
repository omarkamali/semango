package ingest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/omarkamali/semango/internal/config"
)

// ColumnKind represents a coarse column semantic used for decision making.
// For the first iteration we only distinguish Text vs Other.
type ColumnKind string

const (
	TextKind  ColumnKind = "text"
	OtherKind            = "other"
)

// BuildRepresentationForRow converts a single table row (already converted to
// map[string]string) into a Representation suitable for embedding and hybrid
// search. The joined textual payload is columnName: value pairs separated by\n.
func BuildRepresentationForRow(relPath string, rowIdx int, row map[string]string, cfg *config.TabularConfig) (Representation, bool) {
	// Concatenate textual columns
	var builder strings.Builder
	textTokenCount := 0
	for k, v := range row {
		if strings.TrimSpace(v) == "" {
			continue
		}
		builder.WriteString(k)
		builder.WriteString(": ")
		builder.WriteString(v)
		builder.WriteString("\n")
		textTokenCount += len(strings.Fields(v))
	}

	// Skip rows with too little textual signal
	minTokens := 5
	if cfg != nil && cfg.MinTextTokens > 0 {
		minTokens = cfg.MinTextTokens
	}
	if textTokenCount < minTokens {
		return Representation{}, false
	}

	joined := builder.String()
	rep := Representation{
		ID:       ChunkID(relPath, "table_row", int64(rowIdx)),
		Path:     relPath,
		Modality: "table_row",
		Text:     joined,
		Meta:     map[string]string{"row": strconv.Itoa(rowIdx)},
	}

	for k, v := range row {
		rep.Meta[fmt.Sprintf("col.%s", k)] = v
	}

	return rep, true
}
