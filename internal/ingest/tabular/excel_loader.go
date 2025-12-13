package tabular

import (
	"context"

	"github.com/omarkamali/semango/internal/config"
	"github.com/omarkamali/semango/internal/ingest"

	"github.com/xuri/excelize/v2"
)

// ExcelLoader supports .xlsx/.xlsm and .ods files

type ExcelLoader struct {
	cfg config.TabularConfig
}

func NewExcelLoader(cfg config.TabularConfig) *ExcelLoader { return &ExcelLoader{cfg: cfg} }

func (l *ExcelLoader) Extensions() []string { return []string{".xlsx", ".xlsm"} }

func (l *ExcelLoader) Load(ctx context.Context, relPath string, absPath string) ([]ingest.Representation, error) {
	return l.loadXLSX(relPath, absPath)
}

func (l *ExcelLoader) loadXLSX(relPath, absPath string) ([]ingest.Representation, error) {
	f, err := excelize.OpenFile(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheets := f.GetSheetList()
	var all []map[string]string
	for _, sh := range sheets {
		rows, _ := f.GetRows(sh)
		if len(rows) == 0 {
			continue
		}
		headers := rows[0]
		for _, row := range rows[1:] {
			m := map[string]string{}
			for i, cell := range row {
				var key string
				if i < len(headers) && headers[i] != "" {
					key = headers[i]
				} else {
					col, _ := excelize.ColumnNumberToName(i + 1)
					key = col
				}
				m[key] = cell
			}
			all = append(all, m)
		}
	}
	return BuildRepresentations(all, relPath, l.cfg)
}
