package current

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
		Name:  "current",
		Usage: "Print the current version of a release group",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			&cli.StringFlag{
				Name:    "group",
				Usage:   "The group to show release notes for",
				Sources: cli.EnvVars("BUMPER_GROUP"),
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

			groupName, err := shared.GroupFlagOrDefault(ctx, logger, c, cfg)
			if err != nil {
				return err
			}

			cfgGroups := cfg.IndexReleaseGroups()
			group, ok := cfgGroups[groupName]
			if !ok {
				logger.ErrorContext(ctx, "release group not found", slog.String("group", groupName))
				return cmd.Failed(err)
			}

			currentVersion, err := workspace.GetCurrentVersion(ctx, dir, group)
			if err != nil {
				logger.ErrorContext(ctx, "failed to get current version", slog.String("group", groupName), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fmt.Println(currentVersion)

			return nil
		},
	}
}
