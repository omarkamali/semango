package pipeline

import (
	"testing"
	"time"
)

func TestIndexingStatus_Snapshot_inactive(t *testing.T) {
	var s IndexingStatus
	snap := s.Snapshot()

	if snap.Active {
		t.Error("expected inactive")
	}
	if snap.ElapsedMs != 0 {
		t.Errorf("expected 0 elapsed, got %d", snap.ElapsedMs)
	}
	if snap.Progress != 0 {
		t.Errorf("expected 0 progress, got %f", snap.Progress)
	}
	if snap.EtaMs != 0 {
		t.Errorf("expected 0 ETA, got %d", snap.EtaMs)
	}
}

func TestIndexingStatus_Snapshot_active(t *testing.T) {
	var s IndexingStatus
	s.Active.Store(true)
	s.StartedAt.Store(time.Now().Add(-10 * time.Second).UnixNano())
	s.FilesTotal.Store(100)
	s.FilesQueued.Store(50)
	s.FilesDone.Store(40)
	s.FilesFailed.Store(5)
	s.CurrentFile.Store("docs/readme.md")

	snap := s.Snapshot()
	if !snap.Active {
		t.Error("expected active")
	}
	if snap.ElapsedMs < 9000 {
		t.Errorf("expected ~10s elapsed, got %dms", snap.ElapsedMs)
	}
	if snap.FilesTotal != 100 {
		t.Errorf("expected 100 total, got %d", snap.FilesTotal)
	}
	if snap.FilesDone != 40 {
		t.Errorf("expected 40 done, got %d", snap.FilesDone)
	}
	if snap.FilesFailed != 5 {
		t.Errorf("expected 5 failed, got %d", snap.FilesFailed)
	}
	if snap.CurrentFile != "docs/readme.md" {
		t.Errorf("unexpected current file: %s", snap.CurrentFile)
	}
	// Progress = done/total = 40/100 = 0.4
	if snap.Progress < 0.39 || snap.Progress > 0.41 {
		t.Errorf("expected ~0.4 progress, got %f", snap.Progress)
	}
	// ETA should be positive since there are remaining files
	if snap.EtaMs <= 0 {
		t.Errorf("expected positive ETA, got %d", snap.EtaMs)
	}
}

func TestIndexingStatus_Snapshot_zeroTotal(t *testing.T) {
	var s IndexingStatus
	s.Active.Store(true)
	s.StartedAt.Store(time.Now().UnixNano())
	// Total = 0 (haven't seen any files yet)

	snap := s.Snapshot()
	if snap.Progress != 0 {
		t.Errorf("expected 0 progress with zero total, got %f", snap.Progress)
	}
}

func TestIndexingStatus_Snapshot_allDone(t *testing.T) {
	var s IndexingStatus
	s.Active.Store(true)
	s.StartedAt.Store(time.Now().Add(-5 * time.Second).UnixNano())
	s.FilesTotal.Store(10)
	s.FilesQueued.Store(10)
	s.FilesDone.Store(10)
	s.FilesFailed.Store(0)

	snap := s.Snapshot()
	// Progress = 10/10 = 1.0
	if snap.Progress < 0.99 {
		t.Errorf("expected ~1.0 progress, got %f", snap.Progress)
	}
	// ETA should be 0 since queued == done+failed
	// Actually queued(10) - processed(10) = 0 remaining → ETA 0
	if snap.EtaMs != 0 {
		t.Errorf("expected 0 ETA when all done, got %d", snap.EtaMs)
	}
}

func TestManager_Status(t *testing.T) {
	// Verify Status() returns a non-nil pointer to the embedded status.
	m := &Manager{}
	s := m.Status()
	if s == nil {
		t.Fatal("Status() returned nil")
	}
	// Mutate through the pointer and verify.
	s.Active.Store(true)
	snap := m.Status().Snapshot()
	if !snap.Active {
		t.Error("expected active after storing true through pointer")
	}
}
