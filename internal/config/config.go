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
	"gopkg.in/yaml.v3"
	cueErrors "cuelang.org/go/cue/errors"
)

// Config holds the application configuration, loaded from semango.yml
// and environment variables.
// Initially, we'll define a placeholder structure. We'll populate this
// based on spec.md as we implement features.
type Config struct {
	Embedding EmbeddingConfig `yaml:"embedding"`
	Lexical   LexicalConfig   `yaml:"lexical"`
	Reranker  RerankerConfig  `yaml:"reranker"`
	Hybrid    HybridConfig    `yaml:"hybrid"`
	Files     FilesConfig     `yaml:"files"`
	Server    ServerConfig    `yaml:"server"`
	Plugins   []string        `yaml:"plugins"`
	UI        UIConfig        `yaml:"ui"`
	MCP       MCPConfig       `yaml:"mcp"`
}

// EmbeddingConfig matches the 'embedding' section of semango.yml
type EmbeddingConfig struct {
	Provider         string `yaml:"provider" cue:"provider"`
	Model            string `yaml:"model" cue:"model"`
	LocalModelPath   string `yaml:"local_model_path" cue:"local_model_path"`
	BatchSize        int    `yaml:"batch_size" cue:"batch_size"`
	Concurrent       int    `yaml:"concurrent" cue:"concurrent"`
	ModelCacheDir    string `yaml:"model_cache_dir" cue:"model_cache_dir"`
}

// LexicalConfig matches the 'lexical' section of semango.yml
type LexicalConfig struct {
	Enabled   bool    `yaml:"enabled" cue:"enabled"`
	IndexPath string  `yaml:"index_path" cue:"index_path"`
	BM25K1    float64 `yaml:"bm25_k1" cue:"bm25_k1"`
	BM25B     float64 `yaml:"bm25_b" cue:"bm25_b"`
}

// RerankerConfig matches the 'reranker' section of semango.yml
type RerankerConfig struct {
	Enabled              bool   `yaml:"enabled" cue:"enabled"`
	Provider             string `yaml:"provider" cue:"provider"`
	Model                string `yaml:"model" cue:"model"`
	BatchSize            int    `yaml:"batch_size" cue:"batch_size"`
	PerRequestOverride   bool   `yaml:"per_request_override" cue:"per_request_override"`
}

// HybridConfig matches the 'hybrid' section of semango.yml
type HybridConfig struct {
	VectorWeight  float64 `yaml:"vector_weight" cue:"vector_weight"`
	LexicalWeight float64 `yaml:"lexical_weight" cue:"lexical_weight"`
	Fusion        string  `yaml:"fusion" cue:"fusion"`
}

// FilesConfig matches the 'files' section of semango.yml
type FilesConfig struct {
	Include []string `yaml:"include" cue:"include"`
	Exclude []string `yaml:"exclude" cue:"exclude"`
}

// ServerConfig matches the 'server' section of semango.yml
type ServerConfig struct {
	Host    string     `yaml:"host" cue:"host"`
	Port    int        `yaml:"port" cue:"port"`
	Auth    AuthConfig `yaml:"auth" cue:"auth"`
	TLSCert string     `yaml:"tls_cert" cue:"tls_cert"`
	TLSCKey string     `yaml:"tls_key" cue:"tls_key"` // Note: spec.md mentions tls_cert only, but key is usually needed.
}

// AuthConfig matches the 'auth' sub-section of 'server'
type AuthConfig struct {
	Type     string `yaml:"type" cue:"type"`
	TokenEnv string `yaml:"token_env" cue:"token_env"`
}

// UIConfig matches the 'ui' section
type UIConfig struct {
	Enabled bool `yaml:"enabled" cue:"enabled"`
}

// MCPConfig matches the 'mcp' section
type MCPConfig struct {
	Enabled bool `yaml:"enabled" cue:"enabled"`
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
	if configPath == "" {
		configPath = DefaultConfigPath
	}
	if cueSchemaPath == "" {
		cueSchemaPath = DefaultCueSchemaPath
	}

	schemaBytes, err := os.ReadFile(cueSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CUE schema file %s: %w", cueSchemaPath, err)
	}

	yamlData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML data from %s: %w", configPath, err)
	}

	ctx := cuecontext.New()
	schemaVal := ctx.CompileBytes(schemaBytes, cue.Filename(cueSchemaPath))
	if err := schemaVal.Err(); err != nil {
		return nil, fmt.Errorf("failed to compile CUE schema from %s: %w", cueSchemaPath, err)
	}

	cueVal := ctx.Encode(cfg)
	if err := cueVal.Err(); err != nil {
		return nil, fmt.Errorf("failed to encode config struct to CUE value: %w", err)
	}

	configDef := schemaVal.LookupPath(cue.ParsePath("#Config"))
	if !configDef.Exists() {
		return nil, fmt.Errorf("#Config definition not found in CUE schema %s", cueSchemaPath)
	}

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

	cfg.Embedding.ModelCacheDir = expandWithDefault(cfg.Embedding.ModelCacheDir)
	cfg.Lexical.IndexPath = expandWithDefault(cfg.Lexical.IndexPath)

	return &cfg, nil
}

// GetDefaultConfig returns a Config struct populated with default values
// as specified in spec.md.
func GetDefaultConfig() *Config {
	return &Config{
		Embedding: EmbeddingConfig{
			Provider:         "openai",
			Model:            "text-embedding-3-large",
			LocalModelPath:   "models/e5-small.gguf",
			BatchSize:        48,
			Concurrent:       4,
			ModelCacheDir:    "${SEMANGO_MODEL_DIR:=~/.cache/semango}",
		},
		Lexical: LexicalConfig{
			Enabled:   true,
			IndexPath: "./semango/index/bleve",
			BM25K1:    1.2,
			BM25B:     0.75,
		},
		Reranker: RerankerConfig{
			Enabled:              false,
			Provider:             "cohere",
			Model:                "rerank-english-v3.0",
			BatchSize:            32,
			PerRequestOverride: true,
		},
		Hybrid: HybridConfig{
			VectorWeight:  0.7,
			LexicalWeight: 0.3,
			Fusion:        "rrf",
		},
		Files: FilesConfig{
			Include: []string{"**/*.md", "**/*.go", "**/*.{png,jpg,jpeg}", "**/*.pdf"},
			Exclude: []string{".git/**", "node_modules/**", "vendor/**"},
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
	}
}

// WriteDefaultConfig writes the default configuration to the specified path.
// If the path is empty, it uses DefaultConfigPath.
func WriteDefaultConfig(configPath string) error {
	if configPath == "" {
		configPath = DefaultConfigPath
	}

	cfg := GetDefaultConfig()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for config file %s: %w", configPath, err)
		}
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write default config to %s: %w", configPath, err)
	}
	return nil
} 