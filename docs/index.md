---
layout: home
hero:
  name: 🥭 Semango
  text: Hybrid search for your codebase and docs.
  tagline: BM25 + vector search, shipped as a single binary with an embedded UI.
  image:
    src: /mango.svg
    alt: Semango mango icon
  actions:
    - theme: brand
      text: Get started
      link: /guide/quickstart
    - theme: alt
      text: GitHub
      link: https://github.com/omarkamali/semango
features:
  - title: Hybrid by default
    details: Combine lexical (BM25) and semantic (FAISS) retrieval with configurable fusion.
  - title: One binary
    details: Embedded UI + API server + CLI. Run it anywhere, ship it anywhere.
  - title: Bring your own embeddings
    details: Use OpenAI-compatible endpoints or run local ONNX models.
  - title: Ingest real-world data
    details: Markdown, text, code (plain text), PDFs, and tabular files (CSV/JSON/JSONL).
---

## Quick start

```bash
# 1) Create config
semango init

# 2) Index your repo
semango index

# 3) Run the server
semango server
```

Open `http://localhost:8181`.

## What to read next

- Start with the [Guide](/guide/)
- Configuration deep dive: [Configuration](/guide/configuration)
- Embeddings: [Local (ONNX)](/guide/embeddings-local) · [Remote](/guide/embeddings-remote)

---

Built by Omar Kamali (https://omarkamali.com) · Omneity Labs (https://omneitylabs.com)
