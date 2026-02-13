package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFingerprintStore_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "fp.json")

	store := NewFingerprintStore(storePath)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if len(store.items) != 0 {
		t.Errorf("expected empty items, got %d", len(store.items))
	}
}

func TestNewFingerprintStore_LoadExisting(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "fp.json")

	// Write a pre-existing store
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := map[string]FileFingerprint{
		"a.txt": {ModTime: ts, Size: 42},
	}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(storePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewFingerprintStore(storePath)
	if len(store.items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(store.items))
	}
	fp := store.items["a.txt"]
	if fp.Size != 42 {
		t.Errorf("expected size 42, got %d", fp.Size)
	}
	if !fp.ModTime.Equal(ts) {
		t.Errorf("expected mod time %v, got %v", ts, fp.ModTime)
	}
}

func TestNewFingerprintStore_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "fp.json")

	if err := os.WriteFile(storePath, []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewFingerprintStore(storePath)
	if len(store.items) != 0 {
		t.Errorf("expected empty items after corrupt file, got %d", len(store.items))
	}
}

func TestChanged_NewFile(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "fp.json")
	store := NewFingerprintStore(storePath)

	f := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// New file → always changed
	if !store.Changed("hello.txt", f) {
		t.Error("expected Changed=true for a new file")
	}
}

func TestChanged_UnchangedFile(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "fp.json")
	store := NewFingerprintStore(storePath)

	f := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	store.Record("hello.txt", f)

	if store.Changed("hello.txt", f) {
		t.Error("expected Changed=false for unchanged file")
	}
}

func TestChanged_ModifiedFile(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "fp.json")
	store := NewFingerprintStore(storePath)

	f := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	store.Record("hello.txt", f)

	// Modify the file (different size)
	if err := os.WriteFile(f, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !store.Changed("hello.txt", f) {
		t.Error("expected Changed=true after file modification")
	}
}

func TestChanged_MissingFile(t *testing.T) {
	dir := t.TempDir()
	store := NewFingerprintStore(filepath.Join(dir, "fp.json"))

	// File doesn't exist on disk → changed (so loaders can report the error)
	if !store.Changed("ghost.txt", filepath.Join(dir, "ghost.txt")) {
		t.Error("expected Changed=true for non-existent file")
	}
}

func TestRecord_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	store := NewFingerprintStore(filepath.Join(dir, "fp.json"))

	// Recording a non-existent file should be a no-op
	store.Record("ghost.txt", filepath.Join(dir, "ghost.txt"))
	if len(store.items) != 0 {
		t.Error("expected no items recorded for non-existent file")
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	store := NewFingerprintStore(filepath.Join(dir, "fp.json"))

	f := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	store.Record("hello.txt", f)

	if len(store.items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(store.items))
	}

	store.Remove("hello.txt")
	if len(store.items) != 0 {
		t.Errorf("expected 0 items after Remove, got %d", len(store.items))
	}
}

func TestRemove_NonexistentKey(t *testing.T) {
	dir := t.TempDir()
	store := NewFingerprintStore(filepath.Join(dir, "fp.json"))

	// Should not panic or mark dirty
	store.Remove("does-not-exist")
	if store.dirty {
		t.Error("expected dirty=false when removing a non-existent key")
	}
}

func TestSave_NoDirtyNoWrite(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "fp.json")
	store := NewFingerprintStore(storePath)

	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	// File should NOT be created when nothing is dirty
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Error("expected store file to not be created when nothing is dirty")
	}
}

func TestSave_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "fp.json")
	store := NewFingerprintStore(storePath)

	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	store.Record("a.txt", f)

	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	// Load a new store from the same path
	store2 := NewFingerprintStore(storePath)
	if len(store2.items) != 1 {
		t.Fatalf("expected 1 item after reload, got %d", len(store2.items))
	}
	if store2.Changed("a.txt", f) {
		t.Error("expected unchanged after roundtrip")
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "sub", "dir", "fp.json")
	store := NewFingerprintStore(storePath)

	f := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store.Record("x.txt", f)

	if err := store.Save(); err != nil {
		t.Fatalf("Save should create parent dirs: %v", err)
	}
	if _, err := os.Stat(storePath); err != nil {
		t.Fatal("expected store file to exist after Save")
	}
}

func TestDirtyFlag(t *testing.T) {
	dir := t.TempDir()
	store := NewFingerprintStore(filepath.Join(dir, "fp.json"))

	if store.dirty {
		t.Error("expected dirty=false on new store")
	}

	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	store.Record("a.txt", f)
	if !store.dirty {
		t.Error("expected dirty=true after Record")
	}

	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	if store.dirty {
		t.Error("expected dirty=false after Save")
	}

	store.Remove("a.txt")
	if !store.dirty {
		t.Error("expected dirty=true after Remove")
	}
}
