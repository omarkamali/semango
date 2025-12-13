# Local Embedder Guide

See also: [Semango Guide](./SEMANGO_GUIDE.md) for Quickstart, configuration, operations, and advanced usage.

The Local Embedder allows you to use sentence transformer models locally without requiring external API calls. This is ideal for:

- **Privacy**: Keep your data completely local
- **Cost**: No API fees for embedding generation
- **Offline usage**: Work without internet connectivity
- **Custom models**: Use specialized models for your domain

## Supported Models

The local embedder supports popular sentence transformer models from Hugging Face:

### General Purpose Models
- `sentence-transformers/all-MiniLM-L6-v2` - Fast, lightweight (22MB)
- `sentence-transformers/all-mpnet-base-v2` - Balanced performance (420MB)
- `sentence-transformers/all-distilroberta-v1` - Good performance (290MB)

### Multilingual Models
- `sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2`
- `sentence-transformers/paraphrase-multilingual-mpnet-base-v2`

### Domain-Specific Models
- `BAAI/bge-small-en-v1.5` - Optimized for retrieval (130MB)
- `BAAI/bge-base-en-v1.5` - Better quality retrieval (440MB)
- `BAAI/bge-large-en-v1.5` - Best quality retrieval (1.3GB)

### E5 Models (Microsoft)
- `intfloat/e5-small-v2` - Efficient (130MB)
- `intfloat/e5-base-v2` - Balanced (440MB)
- `intfloat/e5-large-v2` - High quality (1.3GB)

## Configuration

### Basic Configuration

```yaml
embedding:
  provider: "local"
  local_model_path: "sentence-transformers/all-MiniLM-L6-v2"
  batch_size: 32
```

### Advanced Configuration

```yaml
embedding:
  provider: "local"
  local_model_path: "BAAI/bge-base-en-v1.5"
  model_cache_dir: "~/.cache/semango/models"  # Where to store downloaded models
  batch_size: 16                              # Adjust based on your memory
  max_length: 512                             # Maximum token length
```

### Using Local Model Files

If you have already downloaded a model or want to use a custom model:

```yaml
embedding:
  provider: "local"
  local_model_path: "/path/to/your/model/directory"
  batch_size: 32
```

## Model Directory Structure

A local model directory should contain:

```
model_directory/
├── config.json              # Model configuration
├── tokenizer.json           # Tokenizer (modern format)
├── vocab.txt               # Vocabulary (legacy format)
├── model.onnx              # ONNX model file
└── 1_Pooling/
    └── config.json         # Pooling configuration
```

## Performance Tuning

### Batch Size
- **Small models** (MiniLM): 32-64
- **Medium models** (base): 16-32  
- **Large models**: 8-16
- **GPU**: Can use larger batch sizes

### Memory Usage
- **MiniLM-L6-v2**: ~100MB RAM
- **mpnet-base-v2**: ~500MB RAM
- **bge-large**: ~1.5GB RAM

### Speed vs Quality Trade-offs

| Model | Size | Speed | Quality | Use Case |
|-------|------|-------|---------|----------|
| all-MiniLM-L6-v2 | 22MB | Fast | Good | Development, testing |
| all-mpnet-base-v2 | 420MB | Medium | Better | General production |
| bge-base-en-v1.5 | 440MB | Medium | Better | Retrieval tasks |
| bge-large-en-v1.5 | 1.3GB | Slow | Best | High-quality retrieval |

## Usage Examples

### Quick Start with MiniLM

```bash
# Create config file
cat > config.yaml << EOF
files:
  root_dir: "./docs"
  include_patterns: ["*.md", "*.txt"]
  chunk_size: 1000
  chunk_overlap: 200

embedding:
  provider: "local"
  local_model_path: "sentence-transformers/all-MiniLM-L6-v2"
  batch_size: 32

search:
  limit: 10
EOF

# Initialize and index
semango init
semango index

# Search
semango search "your query here"
```

### High-Quality Setup with BGE

```bash
# Create config for better quality
cat > config.yaml << EOF
files:
  root_dir: "./documents"
  include_patterns: ["*.pdf", "*.docx", "*.txt"]
  chunk_size: 800
  chunk_overlap: 150

embedding:
  provider: "local"
  local_model_path: "BAAI/bge-base-en-v1.5"
  model_cache_dir: "~/.cache/semango/models"
  batch_size: 16

search:
  limit: 15
  min_score: 0.2
EOF

# Index with high-quality embeddings
semango index
```

## Troubleshooting

### Model Download Issues

If model download fails:

1. **Check internet connection**
2. **Verify model name** - ensure it's in the supported list
3. **Check disk space** - models can be large
4. **Manual download**:
   ```bash
   # Download manually using git-lfs
   git lfs clone https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2
   ```

### Memory Issues

If you get out-of-memory errors:

1. **Reduce batch size**: Try 8, 4, or even 1
2. **Use smaller model**: Switch to MiniLM variants
3. **Increase system memory** or use swap

### Performance Issues

If embedding is too slow:

1. **Use smaller model**: MiniLM is much faster
2. **Increase batch size** (if memory allows)
3. **Reduce chunk size** to process fewer tokens
4. **Consider GPU acceleration** (future feature)

### Configuration Validation

The system will validate your configuration and provide helpful error messages:

```bash
# This will fail with clear error message
semango index
# Error: Local model path is required for local embedder provider

# This will also fail
semango index  
# Error: Unsupported model: invalid/model. Supported models: [...]
```

## Migration from OpenAI

To migrate from OpenAI embeddings to local:

1. **Update configuration**:
   ```yaml
   # Before
   embedding:
     provider: "openai"
     model: "text-embedding-ada-002"
   
   # After  
   embedding:
     provider: "local"
     local_model_path: "sentence-transformers/all-mpnet-base-v2"
   ```

2. **Re-index your documents**:
   ```bash
   # Clear existing index
   rm -rf semango/
   
   # Re-initialize and index
   semango init
   semango index
   ```

3. **Test search quality** and adjust model if needed

## Best Practices

1. **Start small**: Begin with MiniLM for development
2. **Measure quality**: Test search results with your data
3. **Monitor resources**: Watch memory and CPU usage
4. **Cache models**: Reuse downloaded models across projects
5. **Version control**: Pin specific model versions for reproducibility

## Future Enhancements

Planned improvements include:

- **GPU acceleration** using CUDA/ROCm
- **Quantized models** for reduced memory usage
- **Custom model training** integration
- **Batch processing** optimizations
 