package commit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/samber/lo"
	"github.com/urfave/cli/v3"
)

func NewCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "commit",
		Usage: "Commit pending version bumps",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			res, err := shared.Resolve(ctx, logger, shared.DirFlag(c))
			if err != nil {
				return err
			}

			return run(ctx, logger, workspace.ExecRunner{}, workspace.NewGitProvenance(logger, res.Dir), res.Dir, res.Config, os.Stdout)
		},
	}
}

// run commits all pending bumps. Bump files are the release intent: they are
// deleted only once every release group's version and changelog commands have
// succeeded, so a failure partway through leaves the workspace recoverable.
func run(
	ctx context.Context,
	logger *slog.Logger,
	runner workspace.Runner,
	provenance workspace.Provenance,
	dir string,
	cfg *workspace.Config,
	stdout io.Writer,
) error {
	cfgGroups := cfg.IndexReleaseGroups()

	statuses, err := workspace.CollectBumps(ctx, logger, dir, cfg, provenance)
	if err != nil {
		logger.ErrorContext(ctx, "failed to collect pending bumps", slog.String("dir", dir), slog.String("error", err.Error()))
		return cmd.Failed(err)
	}

	if len(statuses) == 0 {
		logger.InfoContext(ctx, "no pending version bumps found", slog.String("dir", dir))

		// Nothing to release, so bump files present here carry no release
		// intent (empty or unknown-group ones) and are safe to clean up,
		// along with any checkpoint from an abandoned batch.
		if err := workspace.DeleteBumps(ctx, dir); err != nil {
			logger.ErrorContext(ctx, "failed to delete bump files", slog.String("dir", dir), slog.String("error", err.Error()))
			return cmd.Failed(err)
		}
		if err := workspace.DeleteCommitCheckpoint(dir); err != nil {
			logger.ErrorContext(ctx, "failed to delete commit checkpoint", slog.String("dir", dir), slog.String("error", err.Error()))
			return cmd.Failed(err)
		}

		return nil
	}

	checkpoint, err := loadCheckpointForBatch(ctx, logger, dir)
	if err != nil {
		return cmd.Failed(err)
	}

	entries := lo.Entries(statuses)
	slices.SortStableFunc(entries, func(e1, e2 lo.Entry[string, *workspace.ReleaseGroupStatus]) int {
		return strings.Compare(e1.Key, e2.Key)
	})

	committedGroups := make([]string, 0, len(statuses))
	for _, entry := range entries {
		groupName, status := entry.Key, entry.Value

		g, ok := cfgGroups[groupName]
		if !ok {
			logger.WarnContext(ctx, "skipping commit for unknown group", slog.String("group", groupName))
			continue
		}

		if status.Level == 0 {
			continue
		}

		if version, released := checkpoint.Released[groupName]; released {
			logger.InfoContext(ctx, "skipping group released by a previous attempt at this batch", slog.String("group", groupName), slog.String("version", version))
			committedGroups = append(committedGroups, groupName)
			continue
		}

		nextVersion, err := workspace.GetNextVersion(ctx, runner, dir, g, status.Level)
		if err != nil {
			logger.ErrorContext(ctx, "failed to get next version", slog.String("group", groupName), slog.String("error", err.Error()))
			return cmd.Failed(fmt.Errorf("release group %s: %w", groupName, err))
		}

		err = commitVersionBump(ctx, runner, dir, g, nextVersion)
		if err != nil {
			logger.ErrorContext(ctx, "failed to commit version bump", slog.String("group", groupName), slog.String("version", nextVersion), slog.String("error", err.Error()))
			return cmd.Failed(fmt.Errorf("release group %s: %w", groupName, err))
		}

		err = commitChangelog(ctx, runner, dir, g, nextVersion, status)
		if err != nil {
			logger.ErrorContext(ctx, "failed to commit changelog", slog.String("group", groupName), slog.String("version", nextVersion), slog.String("error", err.Error()))
			return cmd.Failed(fmt.Errorf("release group %s: %w", groupName, err))
		}

		// Record the group as released before moving on, so a failure in a
		// later group never re-releases this one on retry.
		checkpoint.Released[groupName] = nextVersion
		if err := workspace.SaveCommitCheckpoint(dir, checkpoint); err != nil {
			logger.ErrorContext(ctx, "failed to save commit checkpoint", slog.String("dir", dir), slog.String("error", err.Error()))
			return cmd.Failed(err)
		}

		committedGroups = append(committedGroups, groupName)
	}

	// Every group's commands succeeded; the release intent is consumed.
	if err := workspace.DeleteBumps(ctx, dir); err != nil {
		logger.ErrorContext(ctx, "failed to delete bump files", slog.String("dir", dir), slog.String("error", err.Error()))
		return cmd.Failed(err)
	}
	if err := workspace.DeleteCommitCheckpoint(dir); err != nil {
		logger.ErrorContext(ctx, "failed to delete commit checkpoint", slog.String("dir", dir), slog.String("error", err.Error()))
		return cmd.Failed(err)
	}

	fmt.Fprintln(stdout, strings.Join(committedGroups, "\n"))

	return nil
}

// loadCheckpointForBatch returns the checkpoint from a previously failed
// attempt at the current batch of pending bumps, or a fresh one. A checkpoint
// recorded for a different batch (the bump files changed since the failure)
// is discarded: the modified batch is released from scratch.
func loadCheckpointForBatch(ctx context.Context, logger *slog.Logger, dir string) (*workspace.CommitCheckpoint, error) {
	fingerprint, err := workspace.BumpBatchFingerprint(dir)
	if err != nil {
		logger.ErrorContext(ctx, "failed to fingerprint pending bumps", slog.String("dir", dir), slog.String("error", err.Error()))
		return nil, err
	}

	checkpoint, err := workspace.LoadCommitCheckpoint(dir)
	if err != nil {
		logger.ErrorContext(ctx, "failed to load commit checkpoint", slog.String("dir", dir), slog.String("error", err.Error()))
		return nil, err
	}

	if checkpoint != nil && !maps.Equal(checkpoint.Batch, fingerprint) {
		logger.WarnContext(ctx, "pending bumps changed since the failed commit attempt; releasing the batch from scratch", slog.String("dir", dir))
		checkpoint = nil
	}

	if checkpoint == nil {
		checkpoint = &workspace.CommitCheckpoint{
			Batch:    fingerprint,
			Released: map[string]string{},
		}
	}

	return checkpoint, nil
}

func commitVersionBump(ctx context.Context, runner workspace.Runner, dir string, group workspace.ReleaseGroup, versionStr string) error {
	inv, err := workspace.NewNextInvocation(group, versionStr)
	if err != nil {
		return err
	}

	if err := runner.Run(ctx, dir, inv, os.Stdout); err != nil {
		return fmt.Errorf("execute next version command: %w", err)
	}

	return nil
}

func commitChangelog(ctx context.Context, runner workspace.Runner, dir string, group workspace.ReleaseGroup, versionStr string, status *workspace.ReleaseGroupStatus) error {
	inv, err := workspace.NewChangelogInvocation(group, versionStr, status)
	if err != nil {
		return err
	}

	if err := runner.Run(ctx, dir, inv, os.Stdout); err != nil {
		return fmt.Errorf("execute amend changelog command: %w", err)
	}

	return nil
}
