# Install

## Download a release binary (macOS / Linux)

```bash
curl -L "https://github.com/omarkamali/semango/releases/latest/download/semango_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m).tar.gz" | tar xz
sudo mv semango /usr/local/bin/
```

## Docker

```bash
docker pull ghcr.io/omarkamali/semango:latest
docker run -p 8181:8181 -v $(pwd):/data ghcr.io/omarkamali/semango:latest
```

## Build from source

Requirements: Go 1.23+, Node.js 20+, Yarn, and native deps (FAISS/OpenBLAS/ONNX). See https://github.com/omarkamali/semango/blob/master/BUILD.md.

```bash
git clone https://github.com/omarkamali/semango.git
cd semango
make build
```
