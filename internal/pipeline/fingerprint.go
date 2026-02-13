package pipeline

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileFingerprint stores the mtime and size of a file at the time it was
// last successfully indexed.  If both match on the next run the file is
// skipped, avoiding redundant embedding and index writes.
type FileFingerprint struct {
	ModTime time.Time `json:"mod_time"`
	Size    int64     `json:"size"`
}

// FingerprintStore is a simple JSON-backed store that maps relative file
// paths to their last-indexed fingerprints.
type FingerprintStore struct {
	mu    sync.Mutex
	path  string
	items map[string]FileFingerprint
	dirty bool
}

// NewFingerprintStore loads (or creates) the store at storePath.
func NewFingerprintStore(storePath string) *FingerprintStore {
	fs := &FingerprintStore{
		path:  storePath,
		items: make(map[string]FileFingerprint),
	}
	data, err := os.ReadFile(storePath)
	if err == nil {
		if err := json.Unmarshal(data, &fs.items); err != nil {
			slog.Warn("Corrupt fingerprint store, starting fresh", "path", storePath, "err", err)
			fs.items = make(map[string]FileFingerprint)
		}
	}
	return fs
}

// Changed returns true if the file at absPath has a different mtime or
// size compared to what was stored for relPath.  If we have no record,
// the file is considered changed (i.e. needs indexing).
func (s *FingerprintStore) Changed(relPath, absPath string) bool {
	info, err := os.Stat(absPath)
	if err != nil {
		// Cannot stat → treat as changed so loaders can report the real error.
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fp, ok := s.items[relPath]
	if !ok {
		return true
	}
	return !fp.ModTime.Equal(info.ModTime()) || fp.Size != info.Size()
}

// Record stores the current fingerprint for relPath.
func (s *FingerprintStore) Record(relPath, absPath string) {
	info, err := os.Stat(absPath)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[relPath] = FileFingerprint{
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}
	s.dirty = true
}

// Remove deletes the record for relPath (e.g. file was deleted).
func (s *FingerprintStore) Remove(relPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[relPath]; ok {
		delete(s.items, relPath)
		s.dirty = true
	}
}

// Save persists the store to disk.  It is a no-op if nothing has changed.
func (s *FingerprintStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	s.dirty = false
	return os.WriteFile(s.path, data, 0o644)
}
