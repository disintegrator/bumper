package workspace

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
)

var errNoGitRepository = errors.New("no git repository found")

func isShallowRepo(repo *git.Repository) (bool, error) {
	shallows, err := repo.Storer.Shallow()
	if err != nil {
		return false, err
	}
	return len(shallows) > 0, nil
}

func deepenShallowRepo(ctx context.Context, repo *git.Repository, by int) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("get origin remote: %w", err)
	}

	err = remote.FetchContext(ctx, &git.FetchOptions{
		Depth: by,
		Force: true,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetch with depth %d: %w", by, err)
	}

	return nil
}

func openGitRepository(ctx context.Context, dir string) (*git.Repository, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	switch {
	case errors.Is(err, git.ErrRepositoryNotExists):
		return nil, errNoGitRepository
	case err != nil:
		return nil, fmt.Errorf("open git repository: %w", err)
	}

	isShallow, err := isShallowRepo(repo)
	if err != nil {
		return nil, fmt.Errorf("shallow check: %w", err)
	}

	if isShallow {
		if err := deepenShallowRepo(ctx, repo, 100); err != nil {
			return nil, fmt.Errorf("deepen shallow repo: %w", err)
		}
	}

	return repo, nil
}

type vcsCommit struct {
	SHA  string
	When time.Time
}

func getFirstCommit(repo *git.Repository, filename string) (*vcsCommit, error) {
	abs, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("get absolute path: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get worktree: %w", err)
	}

	relPath, err := filepath.Rel(worktree.Filesystem.Root(), abs)
	if err != nil {
		return nil, fmt.Errorf("get path in worktree: %w", err)
	}

	commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, fmt.Errorf("get commit log: %w", err)
	}
	defer commitIter.Close()

	err = commitIter.ForEach(func(c *object.Commit) error {
		tree, err := c.Tree()
		if err != nil {
			return err
		}

		_, err = tree.File(relPath)
		if err == nil {
			h := c.Hash.String()
			return &foundCommit{sha: h[:min(7, len(h))], date: c.Committer.When}
		}
		return nil
	})

	var ferr *foundCommit
	switch {
	case errors.As(err, &ferr):
		return &vcsCommit{SHA: ferr.sha, When: ferr.date}, nil
	case err != nil:
		return nil, fmt.Errorf("iterate commits: %w", err)
	default:
		return nil, nil
	}
}

type foundCommit struct {
	sha  string
	date time.Time
}

func (c *foundCommit) Error() string {
	return "found git commit"
}
