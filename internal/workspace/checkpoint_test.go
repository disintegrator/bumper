package workspace

import (
	"os"
	"reflect"
	"testing"
)

func setupBumperDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(Dir(dir), 0o755); err != nil {
		t.Fatalf("create .bumper dir: %v", err)
	}

	return dir
}

func TestCommitCheckpointRoundTrip(t *testing.T) {
	dir := setupBumperDir(t)

	loaded, err := LoadCommitCheckpoint(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Fatalf("checkpoint = %#v, want nil when none exists", loaded)
	}

	saved := &CommitCheckpoint{
		Batch:    map[string]string{"bump-a.md": "abc123"},
		Released: map[string]string{"a": "1.1.0"},
	}
	if err := SaveCommitCheckpoint(dir, saved); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err = LoadCommitCheckpoint(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(loaded, saved) {
		t.Errorf("loaded = %#v, want %#v", loaded, saved)
	}

	if err := DeleteCommitCheckpoint(dir); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := DeleteCommitCheckpoint(dir); err != nil {
		t.Errorf("delete missing checkpoint: %v, want nil", err)
	}

	loaded, err = LoadCommitCheckpoint(dir)
	if err != nil || loaded != nil {
		t.Errorf("after delete: checkpoint = %#v, err = %v; want nil, nil", loaded, err)
	}
}

func TestBumpBatchFingerprint(t *testing.T) {
	dir := setupBumperDir(t)

	fingerprint, err := BumpBatchFingerprint(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fingerprint) != 0 {
		t.Errorf("fingerprint = %v, want empty with no bump files", fingerprint)
	}

	if err := os.WriteFile(BumpFilename(dir, "a"), []byte("---\na: minor\n---\n"), 0o644); err != nil {
		t.Fatalf("write bump file: %v", err)
	}

	before, err := BumpBatchFingerprint(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("fingerprint entries = %d, want 1", len(before))
	}

	if err := os.WriteFile(BumpFilename(dir, "a"), []byte("---\na: major\n---\n"), 0o644); err != nil {
		t.Fatalf("rewrite bump file: %v", err)
	}

	after, err := BumpBatchFingerprint(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reflect.DeepEqual(before, after) {
		t.Error("fingerprint unchanged after bump file content changed")
	}
}
