package pre

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

func newEnterCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:      "enter",
		Usage:     "Enter prerelease mode for a release group",
		ArgsUsage: "<group>",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			&cli.StringFlag{
				Name:     "tag",
				Usage:    "The prerelease tag (e.g., alpha, beta, rc)",
				Required: true,
			},
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

			tag := c.String("tag")
			if tag == "" {
				err := errors.New("--tag flag is required")
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

			// Get current version
			currentVersion, err := workspace.GetCurrentVersion(ctx, dir, group)
			if err != nil {
				logger.ErrorContext(ctx, "failed to get current version", slog.String("group", groupName), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// Check if already in prerelease
			existingState := prereleaseState.GetGroupState(groupName)
			if existingState != nil {
				if existingState.Tag == tag {
					fmt.Printf("Group %q is already in prerelease with tag %q\n", groupName, tag)
					return nil
				}
				// Switching tags - keep from_version but reset counter
				fmt.Printf("Switching prerelease tag for %q from %q to %q\n", groupName, existingState.Tag, tag)
				prereleaseState.EnterPrerelease(groupName, tag, existingState.FromVersion)
			} else {
				// Get the base version (strip any prerelease info)
				fromVersion := fmt.Sprintf("%d.%d.%d", currentVersion.Major(), currentVersion.Minor(), currentVersion.Patch())
				prereleaseState.EnterPrerelease(groupName, tag, fromVersion)
			}

			// Save prerelease state
			if err := workspace.SavePrereleaseState(dir, prereleaseState); err != nil {
				logger.ErrorContext(ctx, "failed to save prerelease state", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			state := prereleaseState.GetGroupState(groupName)
			fmt.Printf("Entered prerelease for %q with tag %q\n", groupName, tag)
			fmt.Printf("Next commit will produce: %s-%s.1\n", state.FromVersion, tag)

			return nil
		},
	}
}
