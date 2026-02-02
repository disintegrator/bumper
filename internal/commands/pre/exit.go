package pre

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

func newExitCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:      "exit",
		Usage:     "Exit prerelease mode and graduate to a stable release",
		ArgsUsage: "<group>",
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

			groupName := c.Args().First()
			if groupName == "" {
				err := errors.New("group name is required")
				logger.ErrorContext(ctx, err.Error())
				return cmd.Failed(err)
			}

			// Validate group exists
			cfgGroups := cfg.IndexReleaseGroups()
			group, ok := cfgGroups[groupName]
			if !ok {
				err := fmt.Errorf("release group %q not found", groupName)
				logger.ErrorContext(ctx, err.Error())
				return cmd.Failed(err)
			}

			// Load current prerelease state
			prereleaseState, err := workspace.LoadPrereleaseState(dir)
			if err != nil {
				logger.ErrorContext(ctx, "failed to load prerelease state", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Check if in prerelease
			groupState := prereleaseState.GetGroupState(groupName)
			if groupState == nil {
				err := fmt.Errorf("release group %q is not in prerelease mode", groupName)
				logger.ErrorContext(ctx, err.Error())
				return cmd.Failed(err)
			}

			// Collect processed bumps from prerelease directory
			prereleaseStatuses, err := workspace.CollectPrereleaseBumps(ctx, logger, dir, cfg)
			if err != nil {
				logger.ErrorContext(ctx, "failed to collect prerelease bumps", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Collect pending bumps
			pendingStatuses, err := workspace.CollectBumps(ctx, logger, dir, cfg)
			if err != nil {
				logger.ErrorContext(ctx, "failed to collect pending bumps", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Merge all statuses
			mergedStatuses := workspace.MergeStatuses(prereleaseStatuses, pendingStatuses)

			// Check if there are any bumps to consolidate
			groupStatus, hasGroupBumps := mergedStatuses[groupName]
			if !hasGroupBumps || groupStatus.Level == workspace.BumpLevelNone {
				// No bumps were made during prerelease
				logger.WarnContext(ctx, "no bumps were made during prerelease, cleaning up state")
				prereleaseState.ExitPrerelease(groupName)
				if err := workspace.SavePrereleaseState(dir, prereleaseState); err != nil {
					logger.ErrorContext(ctx, "failed to save prerelease state", slog.String("error", err.Error()))
					return cmd.Failed(err)
				}
				// Clean up any empty prerelease directory
				workspace.DeletePrereleaseBumps(ctx, dir)
				fmt.Printf("Exited prerelease for %q (no changes were made)\n", groupName)
				return nil
			}

			// Calculate stable version based on accumulated bump levels
			accumulatedLevel := workspace.BumpLevelNone
			if accStatus := prereleaseStatuses[groupName]; accStatus != nil {
				accumulatedLevel = accStatus.Level
			}
			pendingLevel := workspace.BumpLevelNone
			if pendStatus := pendingStatuses[groupName]; pendStatus != nil {
				pendingLevel = pendStatus.Level
			}

			stableVersion, _, err := workspace.CalculatePrereleaseVersion(
				groupState.FromVersion,
				groupState.Tag,
				groupState.Counter,
				accumulatedLevel,
				pendingLevel,
			)
			if err != nil {
				logger.ErrorContext(ctx, "failed to calculate version", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Extract the stable version (without prerelease suffix)
			stableVersion, err = workspace.GetStableVersionFromPrerelease(stableVersion)
			if err != nil {
				logger.ErrorContext(ctx, "failed to get stable version", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Delete pending bumps
			if err := workspace.DeleteBumps(ctx, dir); err != nil {
				logger.ErrorContext(ctx, "failed to delete pending bump files", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Delete prerelease bumps
			if err := workspace.DeletePrereleaseBumps(ctx, dir); err != nil {
				logger.ErrorContext(ctx, "failed to delete prerelease bump files", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Build changelog flags with all accumulated changes
			amendFlags := make([]string, 0)
			amendFlags = append(amendFlags, "--group", groupName)

			for _, entry := range groupStatus.MajorLogs {
				amendFlags = append(amendFlags, "--major", entry.Content)
			}
			for _, entry := range groupStatus.MinorLogs {
				amendFlags = append(amendFlags, "--minor", entry.Content)
			}
			for _, entry := range groupStatus.PatchLogs {
				amendFlags = append(amendFlags, "--patch", entry.Content)
			}

			// Commit the stable version
			if err := commitVersionBump(ctx, dir, group, stableVersion); err != nil {
				logger.ErrorContext(ctx, "failed to commit version bump", slog.String("group", groupName), slog.String("version", stableVersion), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Commit the consolidated changelog
			if err := commitChangelog(ctx, dir, group, stableVersion, amendFlags); err != nil {
				logger.ErrorContext(ctx, "failed to commit changelog", slog.String("group", groupName), slog.String("version", stableVersion), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Remove group from prerelease state
			prereleaseState.ExitPrerelease(groupName)
			if err := workspace.SavePrereleaseState(dir, prereleaseState); err != nil {
				logger.ErrorContext(ctx, "failed to save prerelease state", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fmt.Printf("Exited prerelease for %q\n", groupName)
			fmt.Printf("Released version: %s\n", stableVersion)

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
