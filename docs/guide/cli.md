# CLI

Semango ships as a single binary with a small set of commands.

## Common commands

```bash
semango init           # write semango.yml (use -f to change path)
semango index          # index based on semango.yml
semango server         # start UI + REST API
semango search "query" # quick CLI search
semango stats          # print index stats as JSON
semango version        # build/version info
```

## Flags

- `-c, --config` (global): path to `semango.yml` (default: `semango.yml`)
- `-v, --verbose` (global): enable debug logging
- `-f, --file` (init): where to write the config

## Model management

```bash
semango models search bge
semango models download bge-small
semango models list
semango models delete bge-small
```

> The model manager works with ONNX models from the `onnx-models` org on Hugging Face.
