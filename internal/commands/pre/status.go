package pre

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

func newStatusCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:      "status",
		Usage:     "Show the current prerelease state",
		ArgsUsage: "[group]",
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

			// Load current prerelease state
			prereleaseState, err := workspace.LoadPrereleaseState(dir)
			if err != nil {
				logger.ErrorContext(ctx, "failed to load prerelease state", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			// If a specific group was requested
			groupName := c.Args().First()
			if groupName != "" {
				// Validate group exists
				cfgGroups := cfg.IndexReleaseGroups()
				_, ok := cfgGroups[groupName]
				if !ok {
					err := fmt.Errorf("release group %q not found", groupName)
					logger.ErrorContext(ctx, err.Error())
					return cmd.Failed(err)
				}

				state := prereleaseState.GetGroupState(groupName)
				if state == nil {
					fmt.Printf("%s: not in prerelease\n", groupName)
				} else {
					currentVersion := fmt.Sprintf("%s-%s.%d", state.FromVersion, state.Tag, max(state.Counter, 1))
					fmt.Printf("%s: %s (tag: %s, from: %s)\n", groupName, currentVersion, state.Tag, state.FromVersion)
				}
				return nil
			}

			// Show status for all groups
			for _, group := range cfg.Groups {
				state := prereleaseState.GetGroupState(group.Name)
				if state == nil {
					fmt.Printf("%s: not in prerelease\n", group.Name)
				} else {
					currentVersion := fmt.Sprintf("%s-%s.%d", state.FromVersion, state.Tag, max(state.Counter, 1))
					fmt.Printf("%s: %s (tag: %s, from: %s)\n", group.Name, currentVersion, state.Tag, state.FromVersion)
				}
			}

			return nil
		},
	}
}
