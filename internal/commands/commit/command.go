package commit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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

			// Load prerelease state
			prereleaseState, err := workspace.LoadPrereleaseState(dir)
			if err != nil {
				logger.ErrorContext(ctx, "failed to load prerelease state", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Collect pending bumps
			pendingStatuses, err := workspace.CollectBumps(ctx, logger, dir, cfg)
			if err != nil {
				logger.ErrorContext(ctx, "failed to collect pending bumps", slog.String("dir", dir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			if len(pendingStatuses) == 0 {
				logger.InfoContext(ctx, "no pending version bumps found", slog.String("dir", dir))
				return nil
			}

			// Collect accumulated prerelease bumps (for groups in prerelease)
			accumulatedStatuses, err := workspace.CollectPrereleaseBumps(ctx, logger, dir, cfg)
			if err != nil {
				logger.ErrorContext(ctx, "failed to collect prerelease bumps", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Determine which groups have prerelease pending bumps
			prereleaseGroups := make(map[string]bool)
			stableGroups := make(map[string]bool)
			for groupName := range pendingStatuses {
				if prereleaseState.IsInPrerelease(groupName) {
					prereleaseGroups[groupName] = true
				} else {
					stableGroups[groupName] = true
				}
			}

			// Handle prerelease groups: move bump files to prerelease directory
			if len(prereleaseGroups) > 0 {
				if err := workspace.MoveBumpsToPrerelease(ctx, dir); err != nil {
					logger.ErrorContext(ctx, "failed to move bump files to prerelease", slog.String("error", err.Error()))
					return cmd.Failed(err)
				}
			} else {
				// No prerelease groups, delete bump files as before
				if err := workspace.DeleteBumps(ctx, dir); err != nil {
					logger.ErrorContext(ctx, "failed to delete bump files", slog.String("dir", dir), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}
			}

			entries := lo.Entries(pendingStatuses)
			slices.SortStableFunc(entries, func(e1, e2 lo.Entry[string, *workspace.ReleaseGroupStatus]) int {
				return strings.Compare(e1.Key, e2.Key)
			})

			committedGroups := make([]string, 0, len(pendingStatuses))
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

				amendFlags := make([]string, 0, len(status.MajorLogs)+len(status.MinorLogs)+len(status.PatchLogs)+2)
				amendFlags = append(amendFlags, "--group", groupName)

				for _, entry := range status.MajorLogs {
					amendFlags = append(amendFlags, "--major", entry.Content)
				}
				for _, entry := range status.MinorLogs {
					amendFlags = append(amendFlags, "--minor", entry.Content)
				}
				for _, entry := range status.PatchLogs {
					amendFlags = append(amendFlags, "--patch", entry.Content)
				}

				var nextVersion string

				if prereleaseState.IsInPrerelease(groupName) {
					// In prerelease mode - calculate prerelease version
					groupPrereleaseState := prereleaseState.GetGroupState(groupName)

					// Get accumulated level from prerelease bumps
					accumulatedLevel := workspace.BumpLevelNone
					if accStatus, ok := accumulatedStatuses[groupName]; ok {
						accumulatedLevel = accStatus.Level
					}

					// Calculate prerelease version
					var newCounter int
					nextVersion, newCounter, err = workspace.CalculatePrereleaseVersion(
						groupPrereleaseState.FromVersion,
						groupPrereleaseState.Tag,
						groupPrereleaseState.Counter,
						accumulatedLevel,
						status.Level,
					)
					if err != nil {
						logger.ErrorContext(ctx, "failed to calculate prerelease version", slog.String("group", groupName), slog.String("error", err.Error()))
						return cmd.Failed(err)
					}

					// Update counter in prerelease state
					prereleaseState.SetCounter(groupName, newCounter)
				} else {
					// Not in prerelease mode - use regular version calculation
					nextVersion, err = workspace.GetNextVersion(ctx, dir, g, status.Level)
					if err != nil {
						logger.ErrorContext(ctx, "failed to get next version", slog.String("group", groupName), slog.String("error", err.Error()))
						return cmd.Failed(err)
					}
				}

				err = commitVersionBump(ctx, dir, g, nextVersion)
				if err != nil {
					logger.ErrorContext(ctx, "failed to commit version bump", slog.String("group", groupName), slog.String("version", nextVersion), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				err = commitChangelog(ctx, dir, g, nextVersion, amendFlags)
				if err != nil {
					logger.ErrorContext(ctx, "failed to commit changelog", slog.String("group", groupName), slog.String("version", nextVersion), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				committedGroups = append(committedGroups, groupName)
			}

			// Save updated prerelease state if any groups were in prerelease
			if len(prereleaseGroups) > 0 {
				if err := workspace.SavePrereleaseState(dir, prereleaseState); err != nil {
					logger.ErrorContext(ctx, "failed to save prerelease state", slog.String("error", err.Error()))
					return cmd.Failed(err)
				}
			}

			fmt.Println(strings.Join(committedGroups, "\n"))

			return nil
		},
	}
}

func commitVersionBump(ctx context.Context, dir string, group workspace.ReleaseGroup, versionStr string) error {
	if len(group.NextCMD) == 0 {
		return errors.New("no next version command defined for release group")
	}

	nextProg := group.NextCMD[0]
	nextArgs := group.NextCMD[1:]
	cmd := exec.CommandContext(ctx, nextProg, nextArgs...)
	cmd.Dir = dir
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("BUMPER_GROUP=%s", group.Name),
		fmt.Sprintf("BUMPER_GROUP_NEXT_VERSION=%s", versionStr),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("execute next version command: %w", err)
	}

	return nil
}

func commitChangelog(ctx context.Context, dir string, group workspace.ReleaseGroup, versionStr string, flags []string) error {
	if len(group.ChangelogCMD) == 0 {
		return errors.New("no changelog command defined for release group")
	}

	changelogProg := group.ChangelogCMD[0]
	changelogArgs := append(group.ChangelogCMD[1:], flags...)
	cmd := exec.CommandContext(ctx, changelogProg, changelogArgs...)
	cmd.Dir = dir
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("BUMPER_GROUP=%s", group.Name),
		fmt.Sprintf("BUMPER_GROUP_NEXT_VERSION=%s", versionStr),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("execute amend changelog command: %w", err)
	}

	return nil
}
