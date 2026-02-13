package ingest

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/omarkamali/semango/internal/config"
)

// helper to drain the channel into a sorted slice.
func collectCrawl(ctx context.Context, cfg config.FilesConfig) ([]string, error) {
	ch := make(chan string, 100)
	errCh := make(chan error, 1)
	go Crawl(ctx, cfg, ch, errCh)

	var files []string
	for f := range ch {
		files = append(files, f)
	}
	sort.Strings(files)

	select {
	case err := <-errCh:
		return files, err
	default:
		return files, nil
	}
}

func TestCrawl_IncludePattern(t *testing.T) {
	dir := t.TempDir()
	// Crawl uses os.Getwd(), so we chdir into the temp dir for the test.
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	// Create files
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("c"), 0o644)

	cfg := config.FilesConfig{
		Include: []string{"*.txt"},
	}

	files, err := collectCrawl(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "a.txt" {
		t.Errorf("expected [a.txt], got %v", files)
	}
}

func TestCrawl_ExcludePattern(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	os.MkdirAll(filepath.Join(dir, "vendor", "lib"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "vendor", "lib", "x.txt"), []byte("x"), 0o644)

	cfg := config.FilesConfig{
		Include: []string{"**/*.txt"},
		Exclude: []string{"vendor/**"},
	}

	files, err := collectCrawl(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "a.txt" {
		t.Errorf("expected [a.txt], got %v", files)
	}
}

func TestCrawl_NoInclude_AllFiles(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0o644)

	cfg := config.FilesConfig{}

	files, err := collectCrawl(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestCrawl_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	// Create many files to increase the chance the walk is still running when we cancel.
	for i := 0; i < 100; i++ {
		name := filepath.Join(dir, "file"+string(rune('0'+i/10))+string(rune('0'+i%10))+".txt")
		os.WriteFile(name, []byte("data"), 0o644)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan string, 200)
	errCh := make(chan error, 1)
	go Crawl(ctx, config.FilesConfig{}, ch, errCh)

	// Read a few then cancel
	count := 0
	for range ch {
		count++
		if count >= 2 {
			cancel()
			break
		}
	}
	// Drain remaining (channel will close because Crawl returns on ctx error)
	for range ch {
		count++
	}

	// We should have fewer than 100 files because we cancelled early.
	// (On fast systems it might still get all 100, so we just verify no panic
	// and the errChan might have context.Canceled.)
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("unexpected error: %v", err)
		}
	default:
	}
	// The key assertion is that Crawl returned and closed the channel without hanging.
}

func TestCrawl_ContextAlreadyCancelled(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	ch := make(chan string, 100)
	errCh := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		Crawl(ctx, config.FilesConfig{Include: []string{"*.txt"}}, ch, errCh)
		close(done)
	}()

	select {
	case <-done:
		// good — returned promptly
	case <-time.After(5 * time.Second):
		t.Fatal("Crawl did not return after context was cancelled")
	}

	var files []string
	for f := range ch {
		files = append(files, f)
	}
	if len(files) != 0 {
		t.Logf("got %d files despite cancelled context (acceptable race)", len(files))
	}
}

func TestCrawl_SubdirectoryInclude(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.WriteFile(filepath.Join(dir, "docs", "guide.md"), []byte("guide"), 0o644)
	os.WriteFile(filepath.Join(dir, "root.md"), []byte("root"), 0o644)

	cfg := config.FilesConfig{
		Include: []string{"docs/**/*.md"},
	}
	files, err := collectCrawl(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "docs/guide.md" {
		t.Errorf("expected [docs/guide.md], got %v", files)
	}
}

func TestCrawl_ExcludeFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("k"), 0o644)
	os.WriteFile(filepath.Join(dir, "skip.log"), []byte("s"), 0o644)

	cfg := config.FilesConfig{
		Exclude: []string{"*.log"},
	}
	files, err := collectCrawl(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if filepath.Ext(f) == ".log" {
			t.Errorf("did not expect .log file, got %s", f)
		}
	}
}
