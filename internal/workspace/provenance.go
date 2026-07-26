package workspace

import (
	"context"
	"time"
)

// Commit identifies the commit that introduced a bump file.
type Commit struct {
	SHA  string
	When time.Time
}

// Provenance resolves the commit that introduced each bump file. Files that
// cannot be resolved are absent from the returned map; callers treat that as
// "no provenance" rather than an error.
type Provenance interface {
	Resolve(ctx context.Context, bumpFiles []string) (map[string]Commit, error)
}

// FakeProvenance serves canned commits from memory so callers can be tested
// without a git repository.
type FakeProvenance struct {
	Commits map[string]Commit
}

func (f *FakeProvenance) Resolve(_ context.Context, bumpFiles []string) (map[string]Commit, error) {
	info := make(map[string]Commit, len(bumpFiles))
	for _, file := range bumpFiles {
		if commit, ok := f.Commits[file]; ok {
			info[file] = commit
		}
	}

	return info, nil
}
