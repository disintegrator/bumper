package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/storer"
)

var (
	errNoGitRepository = errors.New("no git repository found")
)

func ResolveGitInfoForBumps(ctx context.Context, logger *slog.Logger, repo *git.Repository, bumpFiles []string) (map[string]*vcsCommit, error) {
	info := make(map[string]*vcsCommit, len(bumpFiles))

	pending := make([]string, len(bumpFiles))
	copy(pending, bumpFiles)

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get worktree: %w", err)
	}

	gitRoot := worktree.Filesystem.Root()

	unresolved := []string{}
	for range 10 {
		unresolved = make([]string, 0, len(pending))
		for _, f := range pending {
			relPath, err := filepath.Rel(gitRoot, f)
			if err != nil {
				return nil, fmt.Errorf("get path relative to git root: %w", err)
			}

			commit, err := getFirstCommitWithParent(repo, relPath)
			if err != nil {
				return nil, fmt.Errorf("get first commit for %s: %w", relPath, err)
			}

			if commit == nil {
				unresolved = append(unresolved, f)
			}
		}

		if len(unresolved) == 0 {
			break
		}

		isShallow, err := isShallowRepo(repo)
		if err != nil {
			return nil, fmt.Errorf("check if repo is shallow: %w", err)
		}

		if isShallow {
			if err := deepenShallowRepo(ctx, gitRoot, 50); err != nil {
				return nil, fmt.Errorf("deepen shallow repo: %w", err)
			}
		} else {
			break
		}
	}

	for _, f := range pending {
		relPath, err := filepath.Rel(gitRoot, f)
		if err != nil {
			return nil, fmt.Errorf("get path relative to git root: %w", err)
		}

		commit, err := getFirstCommitWithParent(repo, relPath)
		if err != nil {
			return nil, fmt.Errorf("get first commit for %s: %w", f, err)
		}

		info[f] = &vcsCommit{
			SHA:  commit.Hash.String(),
			When: commit.Committer.When,
		}
	}
	for _, f := range unresolved {
		logger.WarnContext(ctx, "could not resolve git info for bump file", slog.String("file", f))
		delete(info, f)
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

func deepenShallowRepo(ctx context.Context, gitdir string, by int) error {
	cmd := exec.CommandContext(ctx, "git", "fetch", "--deepen", fmt.Sprintf("%d", by))
	cmd.Dir = gitdir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch --deepen %d: %w", by, err)
	}

	return nil
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

type vcsCommit struct {
	SHA  string
	When time.Time
}

func getFirstCommitWithParent(repo *git.Repository, gitFilename string) (*object.Commit, error) {
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
			return nil
		case errors.Is(err, object.ErrParentNotFound):
			// No parent (root commit) - this is where file was added
			commit = c
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
