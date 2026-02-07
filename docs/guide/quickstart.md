# Quickstart

This is the shortest path from zero to a running Semango instance.

```bash
# 1) Create a config file in the current directory
semango init

# 2) Index your content
semango index

# 3) Start the server (UI + REST API)
semango server
```

Open `http://localhost:8181` for the UI.

## Verify the API

```bash
curl -X POST http://localhost:8181/api/v1/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"authentication middleware", "top_k": 5}'
```

> Note: The `server.auth` config is parsed but **not enforced** in the current codebase. If you need auth, place Semango behind a reverse proxy until auth middleware is implemented.
