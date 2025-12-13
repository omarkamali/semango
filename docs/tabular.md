# Tabular Data Ingestion (CSV / JSON* / Parquet)

See also: [Semango Guide](./SEMANGO_GUIDE.md) for Quickstart, configuration, operations, and advanced usage.

Semango 1.1 adds **native support for structured, row-oriented data sources**.  The
same search pipeline now works across code, free-text, *and* analytics files
without extra plugins.

## Supported formats

| Format | Extensions | Reader backend |
|--------|------------|----------------|
| CSV             | `.csv`, `.tsv`           | Go std-lib `encoding/csv` (delimiter configurable)
| JSON array      | `.json`          | streaming `json.Decoder`
| JSON Lines      | `.jsonl`         | `bufio.Scanner`
| Apache Parquet  | `.parquet`       | `github.com/xitongsys/parquet-go` & `parquet-go-source`
| SQLite          | `.sqlite`, `.db` — each table is treated as its own "file" internally; loader streams rows via the `modernc.org/sqlite` driver so no CGO is needed.

> **Tip:** proprietary column separators (TSV, pipe-delimited, …) can be handled
> via a 4-line custom loader that wraps the CSV reader and plugs into the same
> helper.

## How rows become search-able

1.  `internal/ingest/tabular/common.go` inspects the first few thousand rows and
    classifies every column as one of
    `text`, `categorical`, `numeric`, `datetime`, `binary`.
2.  For each physical row we build a joined text snippet:

    ```text
    name: Jane Doe  \n  comment: shipping address missing
    ```

    Only `text` and `categorical` columns participate – numeric & date fields
    are kept **verbatim** in Bleve keyword/typed fields so they can be filtered
    `(meta.col.age < 30)` or boosted `(ship_date > now()-30d)`.
3.  The snippet is sent to the embedding provider and stored as the vector part
    of the chunk.  Metadata holds every original value under `col.<name>` so
    hybrid search and filters "just work".

### Vector explosion control

Key parameters live under the new `tabular:` config node:

```yaml
# semango.yml
...
tabular:
  max_rows_embedded: 50000   # hard cap per file
  sampling: random           # random|stratified
  min_text_tokens: 5         # ignore rows with <N textual tokens
  delimiter: "\t"          # override to support TSV
```

When a file exceeds the cap Semango samples rows (either random reservoir or
simple stratified) and *always* adds two synthetic vectors per file:

* **file-level summary** – "orders_2025.parquet with 2.1 M rows, columns …".
* **schema** – embedding of column names/types only.

These vectors anchor recall for questions about the dataset itself.

## Query examples

```
curl -s -XPOST /search -d '{
  "query": "rows where payment_status \u003d failed", 
  "meta_filters": {"col.payment_status": "failed"}
}' | jq .[0]
```

Hybrid RRF fuses BM25 hits on the keyword filter with semantic neighbours of
"failed payments".

## Trade-offs

Pro | Con
--- | ---
Simple heuristics (no external schema needed) | May mis-classify exotic data types
Embeds only columns likely to matter | Very wide text columns (>5 KB) still produce chunky vectors
Scales to millions of rows via sampling | Per-row embedding may miss cross-row context (use summary vector)

## Extending further

* Add **column-level embeddings** (one vector per unique column) for exploratory
  schema questions.
* Use **IVF-PQ** FAISS index automatically when row count ≫ 1 M to cut RAM.
* Hit an LLM to pick "best" 3-5 columns at query time and re-rank candidates. 