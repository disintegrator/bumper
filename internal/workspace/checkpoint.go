package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// CommitCheckpoint records the progress of a partially failed `bumper commit`
// so a retry can skip release groups whose commands already succeeded. It is
// written after each group completes and removed once the whole batch has
// been released.
type CommitCheckpoint struct {
	// Batch fingerprints the pending bump files (base name -> content hash)
	// the checkpoint belongs to. A retry with different pending bumps is a
	// different batch and must not reuse this checkpoint.
	Batch map[string]string `toml:"batch"`
	// Released maps release group names to the version released for them in
	// a previous attempt at this batch.
	Released map[string]string `toml:"released"`
}

func CommitCheckpointFilename(base string) string {
	return filepath.Join(Dir(base), "commit-checkpoint.toml")
}

// BumpBatchFingerprint hashes the pending bump files, identifying the batch a
// commit attempt operates on.
func BumpBatchFingerprint(dir string) (map[string]string, error) {
	matches, err := filepath.Glob(BumpFilename(dir, "*"))
	if err != nil {
		return nil, fmt.Errorf("glob bump files: %w", err)
	}

	fingerprint := make(map[string]string, len(matches))
	for _, match := range matches {
		content, err := os.ReadFile(match)
		if err != nil {
			return nil, fmt.Errorf("read bump file %s: %w", match, err)
		}
		sum := sha256.Sum256(content)
		fingerprint[filepath.Base(match)] = hex.EncodeToString(sum[:])
	}

	return fingerprint, nil
}

// LoadCommitCheckpoint reads a checkpoint left by a failed commit attempt.
// It returns nil without error when no checkpoint exists.
func LoadCommitCheckpoint(dir string) (*CommitCheckpoint, error) {
	var checkpoint CommitCheckpoint
	_, err := toml.DecodeFile(CommitCheckpointFilename(dir), &checkpoint)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("decode commit checkpoint: %w", err)
	}

	return &checkpoint, nil
}

func SaveCommitCheckpoint(dir string, checkpoint *CommitCheckpoint) error {
	f, err := os.Create(CommitCheckpointFilename(dir))
	if err != nil {
		return fmt.Errorf("create commit checkpoint: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(checkpoint); err != nil {
		return fmt.Errorf("write commit checkpoint: %w", err)
	}

	return nil
}

// DeleteCommitCheckpoint removes the checkpoint; a missing file is not an
// error.
func DeleteCommitCheckpoint(dir string) error {
	err := os.Remove(CommitCheckpointFilename(dir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove commit checkpoint: %w", err)
	}

	return nil
}
