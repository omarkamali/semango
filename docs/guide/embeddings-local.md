# Local embeddings (ONNX)

Semango’s local embedding provider runs **ONNX models** via ONNX Runtime. It does **not** use Python or SentenceTransformers.

## Choose a model

You can use any Hugging Face repo that contains a `.onnx` file. The default config points to the onnx-models org:

```yaml
embedding:
  provider: local
  model: onnx-models/bge-small-en-v1.5-onnx
```

## Download models with the CLI

Semango includes a model manager:

```bash
# Search models on Hugging Face (onnx-models org)
semango models search bge

# Download by alias or full ID
semango models download bge-small

# List installed models
semango models list
```

Models are cached under:

```
~/.cache/semango/models
```

## GPU Acceleration

Semango 🥭 supports GPU acceleration for local embeddings via CUDA. It is enabled by default and will automatically fall back to CPU if no compatible GPU or CUDA runtime is found.

To explicitly configure GPU usage:

```yaml
embedding:
  provider: local
  model: onnx-models/bge-small-en-v1.5-onnx
  # true (default): try GPU, fallback to CPU
  # false: force CPU only
  gpu: true
```

When GPU is successfully enabled, you will see a log entry:
`INFO GPU acceleration (CUDA) enabled for ONNX session`

## Point to a local path

You can also use a local model directory or an explicit `.onnx` file:

```yaml
embedding:
  provider: local
  model: /path/to/model-dir
  # or: /path/to/model.onnx
```

## Output name override

Some ONNX models expose non-standard output names. You can override:

```yaml
embedding:
  provider: local
  model: onnx-models/bge-small-en-v1.5-onnx
  onnx_output_name: sentence_embedding
```
