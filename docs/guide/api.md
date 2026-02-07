# API

Base URL: `http://localhost:8181`

> Auth is **not enforced** in the current server code. The `server.auth` config is parsed but unused.

## POST /api/v1/search

Search indexed content.

**Request**

```json
{
  "query": "string",
  "top_k": 10,
  "filter": "string"
}
```

- `query` is required.
- `top_k` defaults to 10 and is capped at 100.
- `filter` is accepted but **ignored** in the current implementation.

**Example**

```bash
curl -X POST http://localhost:8181/api/v1/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"payment retry logic", "top_k": 5}'
```

**Response (shape)**

```json
{
  "results": [
    {
      "rank": 1,
      "score": 0.95,
      "lexical_score": 0.12,
      "semantic_score": 0.83,
      "modality": "text",
      "document": {
        "path": "docs/README.md",
        "meta": {"source": "TextLoader"}
      },
      "chunk": "...",
      "highlights": {"text": [{"start": 10, "end": 18}]}
    }
  ],
  "query": "payment retry logic",
  "top_k": 5,
  "took": "12.34ms"
}
```

## GET /api/v1/health

```bash
curl http://localhost:8181/api/v1/health
```

**Response**

```json
{
  "status": "healthy",
  "time": "2026-02-07T00:00:00Z"
}
```

## GET /api/v1/stats

Returns index statistics from Bleve/FAISS.

```bash
curl http://localhost:8181/api/v1/stats
```

**Response (shape)**

```json
{
  "total_documents": 120,
  "total_chunks": 640,
  "index_size_bytes": 123456,
  "lexical_path": "./semango/index/bleve",
  "vector_path": "./semango/index/faiss.index",
  "lexical_count": 640,
  "vector_count": 640,
  "lexical_size_bytes": 654321,
  "vector_size_bytes": 987654
}
```

> `total_documents` is currently not populated by the server and may be `0`.
