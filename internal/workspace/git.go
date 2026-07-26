package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/storer"
)

var (
	errNoGitRepository = errors.New("no git repository found")
)

// GitProvenance resolves bump-file provenance from the git repository
// containing Dir, deepening shallow clones as needed. A directory outside any
// git repository degrades to empty results with a warning.
type GitProvenance struct {
	Logger *slog.Logger
	Dir    string
}

func NewGitProvenance(logger *slog.Logger, dir string) *GitProvenance {
	return &GitProvenance{Logger: logger, Dir: dir}
}

var _ Provenance = (*GitProvenance)(nil)

func (g *GitProvenance) Resolve(ctx context.Context, bumpFiles []string) (map[string]Commit, error) {
	info := make(map[string]Commit, len(bumpFiles))

	repo, err := openGitRepository(g.Dir)
	switch {
	case errors.Is(err, errNoGitRepository):
		g.Logger.WarnContext(ctx, "git repository not found", slog.String("dir", g.Dir))
		return info, nil
	case err != nil:
		g.Logger.WarnContext(ctx, "failed to open git repository", slog.String("dir", g.Dir), slog.String("error", err.Error()))
		return info, nil
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get worktree: %w", err)
	}

	gitRoot := worktree.Filesystem().Root()

	pending := slices.Clone(bumpFiles)
	for range 10 {
		unresolved := make([]string, 0, len(pending))
		for _, f := range pending {
			relPath, err := filepath.Rel(gitRoot, f)
			if err != nil {
				return nil, fmt.Errorf("get path relative to git root: %w", err)
			}

			commit, err := getInitialCommitForFile(repo, relPath)
			if err != nil {
				return nil, fmt.Errorf("get first commit for %s: %w", relPath, err)
			}

			if commit == nil {
				unresolved = append(unresolved, f)
			} else {
				info[f] = Commit{
					SHA:  commit.Hash.String(),
					When: commit.Committer.When,
				}
			}
		}

		pending = unresolved
		if len(pending) == 0 {
			break
		}

		isShallow, err := isShallowRepo(repo)
		if err != nil {
			return nil, fmt.Errorf("check if repo is shallow: %w", err)
		}

		if !isShallow {
			break
		}

		repo, err = deepenShallowRepo(ctx, gitRoot, 50)
		if err != nil {
			return nil, fmt.Errorf("deepen shallow repo: %w", err)
		}
	}

	for _, f := range pending {
		g.Logger.WarnContext(ctx, "could not resolve git info for bump file", slog.String("file", f))
	}

	return info, nil
}

func isShallowRepo(repo *git.Repository) (bool, error) {
	shallows, err := repo.Storer.Shallow()
	if err != nil {
		return false, err
	}
	return len(shallows) > 0, nil
}

func deepenShallowRepo(ctx context.Context, gitdir string, by int) (*git.Repository, error) {
	cmd := exec.CommandContext(ctx, "git", "fetch", "--deepen", fmt.Sprintf("%d", by))
	cmd.Dir = gitdir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git fetch --deepen %d: %w", by, err)
	}

	repo, err := openGitRepository(gitdir)
	if err != nil {
		return nil, fmt.Errorf("reopen git repository after deepen: %w", err)
	}

	return repo, nil
}

func openGitRepository(dir string) (*git.Repository, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	switch {
	case errors.Is(err, git.ErrRepositoryNotExists):
		return nil, errNoGitRepository
	case err != nil:
		return nil, fmt.Errorf("open git repository: %w", err)
	}

	return repo, nil
}

func getInitialCommitForFile(repo *git.Repository, gitFilename string) (*object.Commit, error) {
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, fmt.Errorf("get commit log: %w", err)
	}
	defer commitIter.Close()

	var commit *object.Commit
	err = commitIter.ForEach(func(c *object.Commit) error {
		tree, err := c.Tree()
		if err != nil {
			return fmt.Errorf("get commit tree: %w", err)
		}

		_, err = tree.File(gitFilename)
		if err != nil {
			return nil
		}

		p, err := c.Parent(0)
		switch {
		case errors.Is(err, plumbing.ErrObjectNotFound):
			// Parent is beyond a shallow-clone boundary - can't tell whether
			// the file was added here, so leave it unresolved for deepening.
			return nil
		case errors.Is(err, object.ErrParentNotFound):
			// No parent (root commit) - this is where file was added
			commit = c
			return storer.ErrStop
		case err != nil:
			return fmt.Errorf("get commit parent: %w", err)
		}

		parentTree, err := p.Tree()
		if err != nil {
			return fmt.Errorf("get parent tree: %w", err)
		}

		_, err = parentTree.File(gitFilename)
		switch {
		case errors.Is(err, object.ErrFileNotFound):
			// File doesn't exist in parent - this is where it was added!
			commit = c
			return storer.ErrStop
		case err != nil:
			return fmt.Errorf("get file object from parent tree: %w", err)
		}

		// File exists in both commit and parent, keep searching backwards
		return nil
	})
	switch {
	case errors.Is(err, plumbing.ErrObjectNotFound):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("iterate commits: %w", err)
	}

	return commit, nil
}
