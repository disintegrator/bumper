package next

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

func NewCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "next",
		Usage: "Print the next version of a release group",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			&cli.StringFlag{
				Name:     "group",
				Usage:    "The group to show release notes for",
				Required: true,
				Sources:  cli.EnvVars("BUMPER_GROUP"),
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			groupName := c.String("group")
			rawdir := shared.DirFlag(c)
			dir, err := workspace.GetWd(rawdir)
			if err != nil {
				logger.ErrorContext(ctx, "workspace directory not found", slog.String("dir", rawdir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}
			cfg, err := workspace.LoadConfig(dir)
			if err != nil {
				logger.ErrorContext(ctx, "failed to load config", slog.String("dir", dir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			cfgGroups := cfg.IndexReleaseGroups()
			group, ok := cfgGroups[groupName]
			if !ok {
				logger.ErrorContext(ctx, "release group not found", slog.String("group", groupName))
				return cmd.Failed(err)
			}

			statuses, err := workspace.CollectBumps(ctx, logger, dir, cfg)
			if err != nil {
				logger.ErrorContext(ctx, "failed to collect pending bumps", slog.String("dir", dir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			status, ok := statuses[groupName]
			if !ok {
				logger.InfoContext(ctx, "no pending version bump found for group", slog.String("group", groupName))
				return nil
			}

			nextVersion, err := workspace.GetNextVersion(ctx, dir, group, status.Level)
			if err != nil {
				logger.ErrorContext(ctx, "failed to get next version", slog.String("group", groupName), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fmt.Println(nextVersion)

			return nil
		},
	}
}
