package config

import (
	stdlibErrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	// "cuelang.org/go/cue/load" // No longer needed
	cueErrors "cuelang.org/go/cue/errors"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration, loaded from semango.yml
// and environment variables.
// Initially, we'll define a placeholder structure. We'll populate this
// based on spec.md as we implement features.
type Config struct {
	Embedding EmbeddingConfig `json:"embedding" yaml:"embedding"`
	Lexical   LexicalConfig   `json:"lexical" yaml:"lexical"`
	Reranker  RerankerConfig  `json:"reranker" yaml:"reranker"`
	Hybrid    HybridConfig    `json:"hybrid" yaml:"hybrid"`
	Files     FilesConfig     `json:"files" yaml:"files"`
	Server    ServerConfig    `json:"server" yaml:"server"`
	Plugins   []string        `json:"plugins" yaml:"plugins"`
	UI        UIConfig        `json:"ui" yaml:"ui"`
	MCP       MCPConfig       `json:"mcp" yaml:"mcp"`
	Tabular   TabularConfig   `json:"tabular" yaml:"tabular"`
}

// EmbeddingConfig matches the 'embedding' section of semango.yml
type EmbeddingConfig struct {
	Provider       string `json:"provider" yaml:"provider" cue:"provider"`
	Model          string `json:"model" yaml:"model" cue:"model"`
	BatchSize      int    `json:"batch_size" yaml:"batch_size" cue:"batch_size"`
	Concurrent     int    `json:"concurrent" yaml:"concurrent" cue:"concurrent"`
	ModelCacheDir  string `json:"model_cache_dir" yaml:"model_cache_dir" cue:"model_cache_dir"`
	OnnxOutputName string `json:"onnx_output_name" yaml:"onnx_output_name" cue:"onnx_output_name"`
	APIKey         string `json:"api_key" yaml:"api_key" cue:"api_key"`
	APIKeyEnv      string `json:"api_key_env" yaml:"api_key_env" cue:"api_key_env"`
	BaseURL        string `json:"base_url" yaml:"base_url" cue:"base_url"`
	BaseURLEnv     string `json:"base_url_env" yaml:"base_url_env" cue:"base_url_env"`
}

// LexicalConfig matches the 'lexical' section of semango.yml
type LexicalConfig struct {
	Enabled   bool    `json:"enabled" yaml:"enabled" cue:"enabled"`
	IndexPath string  `json:"index_path" yaml:"index_path" cue:"index_path"`
	BM25K1    float64 `json:"bm25_k1" yaml:"bm25_k1" cue:"bm25_k1"`
	BM25B     float64 `json:"bm25_b" yaml:"bm25_b" cue:"bm25_b"`
}

// RerankerConfig matches the 'reranker' section of semango.yml
type RerankerConfig struct {
	Enabled            bool   `json:"enabled" yaml:"enabled" cue:"enabled"`
	Provider           string `json:"provider" yaml:"provider" cue:"provider"`
	Model              string `json:"model" yaml:"model" cue:"model"`
	BatchSize          int    `json:"batch_size" yaml:"batch_size" cue:"batch_size"`
	PerRequestOverride bool   `json:"per_request_override" yaml:"per_request_override" cue:"per_request_override"`
	APIKey             string `json:"api_key" yaml:"api_key" cue:"api_key"`
	APIKeyEnv          string `json:"api_key_env" yaml:"api_key_env" cue:"api_key_env"`
	BaseURL            string `json:"base_url" yaml:"base_url" cue:"base_url"`
	BaseURLEnv     string `json:"base_url_env" yaml:"base_url_env" cue:"base_url_env"`
}

// HybridConfig matches the 'hybrid' section of semango.yml
type HybridConfig struct {
	VectorWeight  float64 `json:"vector_weight" yaml:"vector_weight" cue:"vector_weight"`
	LexicalWeight float64 `json:"lexical_weight" yaml:"lexical_weight" cue:"lexical_weight"`
	Fusion        string  `json:"fusion" yaml:"fusion" cue:"fusion"`
}

// TabularConfig matches the 'tabular' section of semango.yml
type TabularConfig struct {
	MaxRowsEmbedded int    `json:"max_rows_embedded" yaml:"max_rows_embedded" cue:"max_rows_embedded"`
	Sampling        string `json:"sampling" yaml:"sampling" cue:"sampling"`
	MinTextTokens   int    `json:"min_text_tokens" yaml:"min_text_tokens" cue:"min_text_tokens"`
	Delimiter       string `json:"delimiter" yaml:"delimiter" cue:"delimiter"`
}

// FilesConfig matches the 'files' section of semango.yml
type FilesConfig struct {
	Include      []string `json:"include" yaml:"include" cue:"include"`
	Exclude      []string `json:"exclude" yaml:"exclude" cue:"exclude"`
	ChunkSize    int      `json:"chunk_size" yaml:"chunk_size" cue:"chunk_size"`
	ChunkOverlap int      `json:"chunk_overlap" yaml:"chunk_overlap" cue:"chunk_overlap"`
}

// ServerConfig matches the 'server' section of semango.yml
type ServerConfig struct {
	Host    string     `json:"host" yaml:"host" cue:"host"`
	Port    int        `json:"port" yaml:"port" cue:"port"`
	Auth    AuthConfig `json:"auth" yaml:"auth" cue:"auth"`
	TLSCert string     `json:"tls_cert" yaml:"tls_cert" cue:"tls_cert"`
	TLSCKey string     `json:"tls_key" yaml:"tls_key" cue:"tls_key"` // Note: spec.md mentions tls_cert only, but key is usually needed.
}

// AuthConfig matches the 'auth' sub-section of 'server'
type AuthConfig struct {
	Type     string `json:"type" yaml:"type" cue:"type"`
	TokenEnv string `json:"token_env" yaml:"token_env" cue:"token_env"`
}

// UIConfig matches the 'ui' section
type UIConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled" cue:"enabled"`
}

// MCPConfig matches the 'mcp' section
type MCPConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled" cue:"enabled"`
}

// ErrUnknownField is a custom error type for unknown configuration fields.
type ErrUnknownField struct {
	Err error
}

func (e *ErrUnknownField) Error() string {
	return fmt.Sprintf("unknown field in configuration: %v", e.Err)
}

func (e *ErrUnknownField) Unwrap() error {
	return e.Err
}

// DefaultConfigPath is the default path for the configuration file.
const DefaultConfigPath = "semango.yml"
const DefaultCueSchemaPath = "docs/config.cue"

// expandWithDefault expands a string like "${VAR:=default_value}" or "$VAR".
// If VAR is set, its value is used. Otherwise, default_value is used.
// Standard $VAR or ${VAR} without default is also handled by os.ExpandEnv.
var envVarWithDefaultRegex = regexp.MustCompile(`\$\{([^:}]+):=([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return path
}

func expandWithDefault(s string) string {
	result := envVarWithDefaultRegex.ReplaceAllStringFunc(s, func(match string) string {
		expandedSimple := os.ExpandEnv(match)
		if expandedSimple != match && expandedSimple != "" && !strings.Contains(expandedSimple, ":=") {
			return expandPath(expandedSimple)
		}

		parts := envVarWithDefaultRegex.FindStringSubmatch(match)
		var varName, defaultValue string

		if len(parts) > 2 && parts[1] != "" && parts[2] != "" { // ${VAR:=default} form
			varName = parts[1]
			defaultValue = parts[2]
		} else if len(parts) > 3 && parts[3] != "" { // $VAR or ${VAR} form
			varName = parts[3]
			val, _ := os.LookupEnv(varName)
			return expandPath(val)
		} else {
			return expandPath(match)
		}

		value, exists := os.LookupEnv(varName)
		if exists {
			return expandPath(value)
		}

		expandedDefaultValue := expandWithDefault(defaultValue)
		return expandPath(expandedDefaultValue)
	})
	return result
}

// Load attempts to load configuration from the given path and validates it against the CUE schema.
func Load(configPath string, cueSchemaPath string) (*Config, error) {
	// Load environment variables from file if available.
	// Priority: SEMANGO_ENV_FILE (if set) > .env (if present in working directory)
	if customEnv := os.Getenv("SEMANGO_ENV_FILE"); customEnv != "" {
		// Ignore error to keep behavior non-fatal when the file isn't found
		_ = godotenv.Load(customEnv)
	} else {
		_ = godotenv.Load(".env")
	}

	if configPath == "" {
		configPath = DefaultConfigPath
	}
	if cueSchemaPath == "" {
		cueSchemaPath = DefaultCueSchemaPath
	}

	var schemaBytes []byte
	if cueSchemaPath != "" {
		if b, err := os.ReadFile(cueSchemaPath); err == nil {
			schemaBytes = b
		} else {
			schemaBytes = embeddedCueSchema
		}
	} else {
		schemaBytes = embeddedCueSchema
	}

	yamlData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	ctx := cuecontext.New()
	schemaVal := ctx.CompileBytes(schemaBytes, cue.Filename(cueSchemaPath))
	if err := schemaVal.Err(); err != nil {
		return nil, fmt.Errorf("failed to compile CUE schema from %s: %w", cueSchemaPath, err)
	}

	configDef := schemaVal.LookupPath(cue.ParsePath("#Config"))
	if !configDef.Exists() {
		return nil, fmt.Errorf("#Config definition not found in CUE schema %s", cueSchemaPath)
	}

	// 1. Unmarshal YAML to a raw map to avoid Go default values (like 0 for int)
	// which would conflict with CUE constraints (like >=1).
	var rawMap map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &rawMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML data to map from %s: %w", configPath, err)
	}

	// 2. Encode the raw map to CUE
	cueVal := ctx.Encode(rawMap)
	if err := cueVal.Err(); err != nil {
		return nil, fmt.Errorf("failed to encode config map to CUE value: %w", err)
	}

	// 3. Unify with the #Config definition
	instanceVal := configDef.Unify(cueVal)
	if err := instanceVal.Err(); err != nil {
		var cueErrList cueErrors.Error
		if stdlibErrors.As(err, &cueErrList) {
			for _, einzelneError := range cueErrors.Errors(cueErrList) {
				if strings.Contains(cueErrors.Details(einzelneError, nil), "field not allowed") ||
					strings.Contains(cueErrors.Details(einzelneError, nil), "is not a field in") {
					return nil, &ErrUnknownField{Err: err}
				}
			}
		}
		return nil, fmt.Errorf("failed to unify CUE #Config definition with config data from %s: %w", configPath, err)
	}

	// 4. Validate and ensure concrete values (fills in defaults)
	if err := instanceVal.Validate(cue.Concrete(true)); err != nil {
		var cueErrList cueErrors.Error
		if stdlibErrors.As(err, &cueErrList) {
			for _, einzelneError := range cueErrors.Errors(cueErrList) {
				if strings.Contains(cueErrors.Details(einzelneError, nil), "field not allowed") ||
					strings.Contains(cueErrors.Details(einzelneError, nil), "is not a field in") {
					return nil, &ErrUnknownField{Err: err}
				}
			}
		}
		return nil, fmt.Errorf("CUE validation failed for %s (schema %s, def #Config): %w. Exit code 78 may be required.", configPath, cueSchemaPath, err)
	}

	// 5. Decode the unified value (with defaults applied) back into our Config struct
	var cfg Config
	if err := instanceVal.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode unified CUE value into Config struct: %w", err)
	}

	cfg.Embedding.ModelCacheDir = expandWithDefault(cfg.Embedding.ModelCacheDir)
	cfg.Lexical.IndexPath = expandWithDefault(cfg.Lexical.IndexPath)

	return &cfg, nil
}

// GetDefaultConfig returns a Config struct populated with default values
// as specified in spec.md.
func GetDefaultConfig() *Config {
	return &Config{
		Embedding: EmbeddingConfig{
			Provider:       "local",
			Model:          "onnx-models/all-MiniLM-L6-v2-onnx",
			BatchSize:      48,
			Concurrent:     4,
			ModelCacheDir:  "${SEMANGO_MODEL_DIR:=~/.cache/semango}",
		},
		Lexical: LexicalConfig{
			Enabled:   true,
			IndexPath: "./semango/index/bleve",
			BM25K1:    1.2,
			BM25B:     0.75,
		},
		Reranker: RerankerConfig{
			Enabled:            false,
			Provider:           "openai",
			Model:              "rerank-english-v3.0",
			BatchSize:          32,
			PerRequestOverride: true,
		},
		Hybrid: HybridConfig{
			VectorWeight:  0.7,
			LexicalWeight: 0.3,
			Fusion:        "linear",
		},
		Files: FilesConfig{
			Include:      []string{"**/*.md", "**/*.go", "**/*.{png,jpg,jpeg}", "**/*.pdf", "**/*.csv", "**/*.json", "**/*.jsonl", "**/*.parquet"},
			Exclude:      []string{".git/**", "node_modules/**", "vendor/**"},
			ChunkSize:    1000,
			ChunkOverlap: 200,
		},
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8181,
			Auth: AuthConfig{
				Type:     "token",
				TokenEnv: "SEMANGO_TOKENS",
			},
			TLSCert: "",
			TLSCKey: "", // Assuming empty default for key as well
		},
		Plugins: []string{
			"plugins/",
			"../shared/my_custom.so",
		},
		UI: UIConfig{
			Enabled: true,
		},
		MCP: MCPConfig{
			Enabled: true,
		},
		Tabular: TabularConfig{
			MaxRowsEmbedded: 1000,
			Sampling:        "random",
			MinTextTokens:   5,
			Delimiter:       "",
		},
	}
}

// DefaultConfigYAML is a template for the default configuration file with comments.
const DefaultConfigYAML = `# Semango configuration file

# Embedding settings
embedding:
  # Provider for embeddings: local, openai
  provider: local
  # Model name for the selected provider (Hugging Face ID for local provider)
  model: onnx-models/bge-small-en-v1.5-onnx
  batch_size: 48
  concurrent: 4
  # Directory where models are cached
  model_cache_dir: "${SEMANGO_MODEL_DIR:=~/.cache/semango}"
  # For openai provider:
  # api_key: "your-api-key"
  # api_key_env: "OPENAI_API_KEY"
  # base_url: "https://api.openai.com/v1"
  # base_url_env: "OPENAI_BASE_URL"

# Lexical search settings (BM25)
lexical:
  enabled: true
  index_path: ./semango/index/bleve
  bm25_k1: 1.2
  bm25_b: 0.75

# Reranker settings
reranker:
  enabled: false
  provider: openai
  model: rerank-english-v3.0
  batch_size: 32
  per_request_override: true
  # For openai provider:
  # api_key: "your-api-key"
  # api_key_env: "OPENAI_API_KEY"
  # base_url: "https://api.openai.com/v1"
  # base_url_env: "OPENAI_BASE_URL"

# Hybrid search merging settings
hybrid:
  vector_weight: 0.7
  lexical_weight: 0.3
  fusion: linear

# File indexing settings
files:
  include:
    - "**/*.md"
    - "**/*.go"
    - "**/*.{png,jpg,jpeg}"
    - "**/*.pdf"
    - "**/*.csv"
    - "**/*.json"
    - "**/*.jsonl"
    - "**/*.parquet"
  exclude:
    - ".git/**"
    - "node_modules/**"
    - "vendor/**"
  chunk_size: 1000
  chunk_overlap: 200

# Server settings
server:
  host: 0.0.0.0
  port: 8181
  auth:
    type: token
    token_env: SEMANGO_TOKENS
  tls_cert: ""
  tls_key: ""

# Plugin settings
plugins:
  - plugins/
  - ../shared/my_custom.so

# Web UI settings
ui:
  enabled: true

# Model Context Protocol (MCP) settings
mcp:
  enabled: true

# Tabular data settings
tabular:
  max_rows_embedded: 1000
  sampling: random
  min_text_tokens: 5
  delimiter: ""
`

// WriteDefaultConfig writes the default configuration to the specified path.
// If the path is empty, it uses DefaultConfigPath.
func WriteDefaultConfig(configPath string) error {
	if configPath == "" {
		configPath = DefaultConfigPath
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for config file %s: %w", configPath, err)
		}
	}

	if err := os.WriteFile(configPath, []byte(DefaultConfigYAML), 0644); err != nil {
		return fmt.Errorf("failed to write default config to %s: %w", configPath, err)
	}
	return nil
}
