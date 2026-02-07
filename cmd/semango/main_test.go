package main

import (
	"bytes"
	"testing"
)

func TestRootHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("rootCmd.Execute failed: %v", err)
	}

	output := buf.String()
	expected := []string{"install", "models", "init", "index", "search", "server", "version"}
	for _, cmd := range expected {
		if !contains(output, cmd) {
			t.Errorf("Expected root help to contain command: %s", cmd)
		}
	}
}

func TestModelsHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"models", "--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("rootCmd.Execute failed: %v", err)
	}

	output := buf.String()
	expected := []string{"search", "list", "download", "delete"}
	for _, cmd := range expected {
		if !contains(output, cmd) {
			t.Errorf("Expected models help to contain subcommand: %s", cmd)
		}
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
