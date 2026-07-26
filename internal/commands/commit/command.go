package commit

import (
	"context"
	"fmt"
	"log/slog"
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
			rawdir := shared.DirFlag(c)
			dir, err := workspace.GetWd(rawdir)
			if err != nil {
				logger.ErrorContext(ctx, "workspace directory not found", slog.String("dir", rawdir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			cfg, err := shared.LoadConfig(ctx, logger, dir)
			if err != nil {
				return err
			}

			cfgGroups := cfg.IndexReleaseGroups()

			statuses, err := workspace.CollectBumps(ctx, logger, dir, cfg, workspace.NewGitProvenance(logger, dir))
			if err != nil {
				logger.ErrorContext(ctx, "failed to collect pending bumps", slog.String("dir", dir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			if err := workspace.DeleteBumps(ctx, dir); err != nil {
				logger.ErrorContext(ctx, "failed to delete bump files", slog.String("dir", dir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			if len(statuses) == 0 {
				logger.InfoContext(ctx, "no pending version bumps found", slog.String("dir", dir))
				return nil
			}

			entries := lo.Entries(statuses)
			slices.SortStableFunc(entries, func(e1, e2 lo.Entry[string, *workspace.ReleaseGroupStatus]) int {
				return strings.Compare(e1.Key, e2.Key)
			})

			runner := workspace.ExecRunner{}
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

				nextVersion, err := workspace.GetNextVersion(ctx, runner, dir, g, status.Level)
				if err != nil {
					logger.ErrorContext(ctx, "failed to get next version", slog.String("group", groupName), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				err = commitVersionBump(ctx, runner, dir, g, nextVersion)
				if err != nil {
					logger.ErrorContext(ctx, "failed to commit version bump", slog.String("group", groupName), slog.String("version", nextVersion), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				err = commitChangelog(ctx, runner, dir, g, nextVersion, status)
				if err != nil {
					logger.ErrorContext(ctx, "failed to commit changelog", slog.String("group", groupName), slog.String("version", nextVersion), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				committedGroups = append(committedGroups, groupName)
			}

			fmt.Println(strings.Join(committedGroups, "\n"))

			return nil
		},
	}
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
