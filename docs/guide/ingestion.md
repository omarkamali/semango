# Ingestion

Semango indexes files using built-in loaders. This page reflects what is **actually implemented** in the current codebase.

## Supported file types

### Text
- `.txt`, `.md`, `.go`

### Code (treated as plain text)
The code loader detects language from file extension and stores code as text. Tree-sitter parsing is **not implemented** yet.

Large code files (> 5MB) are skipped.

Supported extensions:

```
.go .js .ts .py .jsx .tsx .java .c .cpp .h .hpp .rs .rb .php .cs .swift .kt .scala
```

### PDF
- `.pdf` (text extraction)

### Tabular
- `.csv`, `.tsv`
- `.json` (array of objects)
- `.jsonl`

## Not supported (yet)

- Images, audio, and OCR-based image ingestion
- Parquet and SQLite
- Structured code parsing (AST)

## Chunking

Chunking is applied to text and PDF content:

```yaml
files:
  chunk_size: 1000
  chunk_overlap: 200
```

For tabular files, Semango converts each row to a text snippet and embeds it (subject to `tabular` limits).
