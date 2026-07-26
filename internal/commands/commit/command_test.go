package commit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/workspace"
)

// scriptedRunner serves current versions and fails the version-bump command
// for one designated group.
type scriptedRunner struct {
	failGroup string
	calls     []workspace.GroupInvocation
}

func (r *scriptedRunner) Run(_ context.Context, _ string, inv workspace.GroupInvocation, stdout io.Writer) error {
	r.calls = append(r.calls, inv)

	if r.failGroup != "" && inv.Verb == workspace.VerbNext && slices.Contains(inv.Env, "BUMPER_GROUP="+r.failGroup) {
		return errors.New("boom")
	}

	if inv.Verb == workspace.VerbCurrent {
		if _, err := io.WriteString(stdout, "1.0.0\n"); err != nil {
			return err
		}
	}

	return nil
}

func testGroup(name string) workspace.ReleaseGroup {
	return workspace.ReleaseGroup{
		Name:         name,
		ChangelogCMD: []string{"true"},
		CatCMD:       []string{"true"},
		CurrentCMD:   []string{"true"},
		NextCMD:      []string{"true"},
	}
}

// setupPendingBumps creates a workspace directory with one pending minor bump
// file per group name.
func setupPendingBumps(t *testing.T, groups ...string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(workspace.Dir(dir), 0o755); err != nil {
		t.Fatalf("create .bumper dir: %v", err)
	}
	for _, group := range groups {
		content := "---\n" + group + ": minor\n---\n\nchange for " + group + "\n"
		if err := os.WriteFile(workspace.BumpFilename(dir, group), []byte(content), 0o644); err != nil {
			t.Fatalf("write bump file: %v", err)
		}
	}

	return dir
}

func pendingBumpFiles(t *testing.T, dir string) []string {
	t.Helper()

	matches, err := filepath.Glob(workspace.BumpFilename(dir, "*"))
	if err != nil {
		t.Fatalf("glob bump files: %v", err)
	}

	return matches
}

func TestRunFailureKeepsBumpFiles(t *testing.T) {
	dir := setupPendingBumps(t, "a", "b", "c")
	cfg := &workspace.Config{Groups: []workspace.ReleaseGroup{
		testGroup("a"), testGroup("b"), testGroup("c"),
	}}
	runner := &scriptedRunner{failGroup: "b"}
	logger := slog.New(slog.DiscardHandler)
	var stdout bytes.Buffer

	err := run(t.Context(), logger, runner, &workspace.FakeProvenance{}, dir, cfg, &stdout)
	if err == nil {
		t.Fatal("expected an error when the second group fails")
	}

	var cmdErr *cmd.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected a CommandError, got %T: %v", err, err)
	}
	cause := cmdErr.Unwrap()
	if cause == nil {
		t.Fatal("CommandError wraps a nil cause")
	}
	if !strings.Contains(cause.Error(), "release group b") {
		t.Errorf("error = %q, want it to name release group b", cause.Error())
	}

	if got := pendingBumpFiles(t, dir); len(got) != 3 {
		t.Errorf("bump files remaining = %d, want all 3 preserved: %v", len(got), got)
	}

	// Group a completed before the failure - its intent is preserved anyway.
	sawGroupAChangelog := slices.ContainsFunc(runner.calls, func(inv workspace.GroupInvocation) bool {
		return inv.Verb == workspace.VerbChangelog && slices.Contains(inv.Env, "BUMPER_GROUP=a")
	})
	if !sawGroupAChangelog {
		t.Error("expected group a to have been fully committed before the failure")
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want no committed groups reported on failure", stdout.String())
	}
}

func TestRunSuccessDeletesBumpFiles(t *testing.T) {
	dir := setupPendingBumps(t, "a", "b", "c")
	cfg := &workspace.Config{Groups: []workspace.ReleaseGroup{
		testGroup("a"), testGroup("b"), testGroup("c"),
	}}
	runner := &scriptedRunner{}
	logger := slog.New(slog.DiscardHandler)
	var stdout bytes.Buffer

	if err := run(t.Context(), logger, runner, &workspace.FakeProvenance{}, dir, cfg, &stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := pendingBumpFiles(t, dir); len(got) != 0 {
		t.Errorf("bump files remaining = %v, want none after a successful commit", got)
	}

	if got, want := stdout.String(), "a\nb\nc\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

// Bump files that carry no release intent (e.g. from bump --empty) are still
// cleaned up when there is nothing to release.
func TestRunNoEffectiveBumpsStillCleansUp(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(workspace.Dir(dir), 0o755); err != nil {
		t.Fatalf("create .bumper dir: %v", err)
	}
	if err := os.WriteFile(workspace.BumpFilename(dir, "empty"), []byte("---\n---\n"), 0o644); err != nil {
		t.Fatalf("write empty bump file: %v", err)
	}

	cfg := &workspace.Config{Groups: []workspace.ReleaseGroup{testGroup("a")}}
	runner := &scriptedRunner{}
	logger := slog.New(slog.DiscardHandler)

	if err := run(t.Context(), logger, runner, &workspace.FakeProvenance{}, dir, cfg, io.Discard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := pendingBumpFiles(t, dir); len(got) != 0 {
		t.Errorf("bump files remaining = %v, want empty bump cleaned up", got)
	}
	if len(runner.calls) != 0 {
		t.Errorf("runner calls = %d, want none when nothing is released", len(runner.calls))
	}
}
