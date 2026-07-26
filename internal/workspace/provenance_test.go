package workspace

import (
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
)

func TestFakeProvenance(t *testing.T) {
	known := Commit{SHA: "abc1234def", When: time.Unix(1700000000, 0)}
	prov := &FakeProvenance{
		Commits: map[string]Commit{
			"/repo/.bumper/bump-known.md": known,
		},
	}

	info, err := prov.Resolve(t.Context(), []string{
		"/repo/.bumper/bump-known.md",
		"/repo/.bumper/bump-unknown.md",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]Commit{"/repo/.bumper/bump-known.md": known}
	if !reflect.DeepEqual(info, want) {
		t.Errorf("info = %#v, want %#v", info, want)
	}
}

// initTestRepo initializes a git repository with commit signing disabled so
// tests do not depend on the developer's global git configuration.
func initTestRepo(t *testing.T, dir string) *git.Repository {
	t.Helper()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init git repository: %v", err)
	}

	cfg, err := repo.Config()
	if err != nil {
		t.Fatalf("read repository config: %v", err)
	}
	cfg.Raw.Section("commit").SetOption("gpgsign", "false")
	if err := repo.SetConfig(cfg); err != nil {
		t.Fatalf("write repository config: %v", err)
	}

	return repo
}

func writeBumpFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()

	if err := os.MkdirAll(Dir(dir), 0o755); err != nil {
		t.Fatalf("create .bumper dir: %v", err)
	}

	path := BumpFilename(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write bump file: %v", err)
	}

	return path
}

const bumpContent = "---\napi: minor\n---\n\nAdded something\n"

// Regression test: running commands in a directory that is not a git
// repository must degrade to a warning and changelog entries without commit
// prefixes instead of panicking on a nil repository.
func TestCollectBumpsNonGitDirectory(t *testing.T) {
	dir := t.TempDir()
	writeBumpFile(t, dir, "test", bumpContent)

	logger := slog.New(slog.DiscardHandler)
	cfg := &Config{Groups: []ReleaseGroup{{Name: "api"}}}

	statuses, err := CollectBumps(t.Context(), logger, dir, cfg, NewGitProvenance(logger, dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status, ok := statuses["api"]
	if !ok {
		t.Fatal("expected a status for group api")
	}
	if status.Level != BumpLevelMinor {
		t.Errorf("level = %v, want %v", status.Level, BumpLevelMinor)
	}
	if len(status.MinorLogs) != 1 {
		t.Fatalf("minor logs = %d, want 1", len(status.MinorLogs))
	}
	if got := status.MinorLogs[0].Content; got != "Added something" {
		t.Errorf("content = %q, want %q (no commit prefix)", got, "Added something")
	}
}

// Regression test: a bump file introduced in the repository's root commit
// must resolve to that commit instead of panicking on a nil parent.
func TestGitProvenanceRootCommit(t *testing.T) {
	dir := t.TempDir()
	repo := initTestRepo(t, dir)

	path := writeBumpFile(t, dir, "test", bumpContent)

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("get worktree: %v", err)
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		t.Fatalf("relative bump path: %v", err)
	}
	if _, err := worktree.Add(rel); err != nil {
		t.Fatalf("git add: %v", err)
	}
	sig := &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(1700000000, 0)}
	hash, err := worktree.Commit("root commit with bump", &git.CommitOptions{Author: sig, Committer: sig})
	if err != nil {
		t.Fatalf("git commit: %v", err)
	}

	logger := slog.New(slog.DiscardHandler)
	info, err := NewGitProvenance(logger, dir).Resolve(t.Context(), []string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	commit, ok := info[path]
	if !ok {
		t.Fatal("expected bump file to resolve to the root commit")
	}
	if commit.SHA != hash.String() {
		t.Errorf("sha = %q, want %q", commit.SHA, hash.String())
	}
}

// A bump file added after the root commit resolves to the commit that
// introduced it, not an ancestor.
func TestGitProvenanceLaterCommit(t *testing.T) {
	dir := t.TempDir()
	repo := initTestRepo(t, dir)

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("get worktree: %v", err)
	}

	sig := &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(1700000000, 0)}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if _, err := worktree.Add("README.md"); err != nil {
		t.Fatalf("git add README: %v", err)
	}
	if _, err := worktree.Commit("root commit", &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatalf("git commit root: %v", err)
	}

	path := writeBumpFile(t, dir, "test", bumpContent)
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		t.Fatalf("relative bump path: %v", err)
	}
	if _, err := worktree.Add(rel); err != nil {
		t.Fatalf("git add bump: %v", err)
	}
	hash, err := worktree.Commit("add bump", &git.CommitOptions{Author: sig, Committer: sig})
	if err != nil {
		t.Fatalf("git commit bump: %v", err)
	}

	logger := slog.New(slog.DiscardHandler)
	info, err := NewGitProvenance(logger, dir).Resolve(t.Context(), []string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	commit, ok := info[path]
	if !ok {
		t.Fatal("expected bump file to resolve")
	}
	if commit.SHA != hash.String() {
		t.Errorf("sha = %q, want %q", commit.SHA, hash.String())
	}
}

// CollectBumps prefixes changelog entries with the short commit SHA when
// provenance resolves, exercising the seam with the fake adapter.
func TestCollectBumpsWithFakeProvenance(t *testing.T) {
	dir := t.TempDir()
	path := writeBumpFile(t, dir, "test", bumpContent)

	logger := slog.New(slog.DiscardHandler)
	cfg := &Config{Groups: []ReleaseGroup{{Name: "api"}}}
	prov := &FakeProvenance{
		Commits: map[string]Commit{
			path: {SHA: "abc1234def5678", When: time.Unix(1700000000, 0)},
		},
	}

	statuses, err := CollectBumps(t.Context(), logger, dir, cfg, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status, ok := statuses["api"]
	if !ok {
		t.Fatal("expected a status for group api")
	}
	if len(status.MinorLogs) != 1 {
		t.Fatalf("minor logs = %d, want 1", len(status.MinorLogs))
	}
	entry := status.MinorLogs[0]
	if want := "abc1234: Added something"; entry.Content != want {
		t.Errorf("content = %q, want %q", entry.Content, want)
	}
	if want := time.Unix(1700000000, 0).UnixNano(); entry.Timestamp != want {
		t.Errorf("timestamp = %d, want %d", entry.Timestamp, want)
	}
	if entry.Commit != "abc1234" {
		t.Errorf("commit = %q, want %q", entry.Commit, "abc1234")
	}
}
