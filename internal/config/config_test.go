package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadAndExpansion(t *testing.T) {
	tempDir := t.TempDir()
	tempConfigPath := filepath.Join(tempDir, "semango.yml")
	tempCuePath := filepath.Join(tempDir, "config.cue")

	// Write a permissive CUE schema for all fields except the ones we want to check strictly
	cueSchema := `
package config
#Config: {
  embedding: {
    model_cache_dir: string
    ...
  }
  lexical: {
    index_path: string
    ...
  }
  reranker?: _
  hybrid?: _
  files?: _
  server?: _
  plugins?: _
  ui?: _
  mcp?: _
  tabular?: _
}
`
	if err := os.WriteFile(tempCuePath, []byte(cueSchema), 0644); err != nil {
		t.Fatalf("failed to write temp cue schema: %v", err)
	}

	// Write a config YAML with env var and ~
	configYAML := `embedding:
  model_cache_dir: "${TEST_SEMANGO_DIR:=~/test_semango_cache}"
lexical:
  index_path: "./test_index"
`
	if err := os.WriteFile(tempConfigPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	// Unset TEST_SEMANGO_DIR to test default expansion
	_ = os.Unsetenv("TEST_SEMANGO_DIR")

	cfg, err := Load(tempConfigPath, tempCuePath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	expectedCacheDir := filepath.Join(home, "test_semango_cache")
	if cfg.Embedding.ModelCacheDir != expectedCacheDir {
		t.Errorf("expected ModelCacheDir=%q, got %q", expectedCacheDir, cfg.Embedding.ModelCacheDir)
	}
	if cfg.Lexical.IndexPath != "./test_index" {
		t.Errorf("expected IndexPath=./test_index, got %q", cfg.Lexical.IndexPath)
	}

	// Now set TEST_SEMANGO_DIR and test override
	os.Setenv("TEST_SEMANGO_DIR", "/tmp/override_semango")
	cfg2, err := Load(tempConfigPath, tempCuePath)
	if err != nil {
		t.Fatalf("Load with env override failed: %v", err)
	}
	if cfg2.Embedding.ModelCacheDir != "/tmp/override_semango" {
		t.Errorf("expected ModelCacheDir=/tmp/override_semango, got %q", cfg2.Embedding.ModelCacheDir)
	}
}
