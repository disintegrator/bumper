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
				Name:    "group",
				Usage:   "The group to show release notes for",
				Sources: cli.EnvVars("BUMPER_GROUP"),
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			res, err := shared.Resolve(ctx, logger, shared.DirFlag(c))
			if err != nil {
				return err
			}

			group, err := res.Group(ctx, logger, c.String("group"))
			if err != nil {
				return err
			}

			statuses, err := workspace.CollectBumps(ctx, logger, res.Dir, res.Config, workspace.NewGitProvenance(logger, res.Dir))
			if err != nil {
				logger.ErrorContext(ctx, "failed to collect pending bumps", slog.String("dir", res.Dir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			status, ok := statuses[group.Name]
			if !ok {
				logger.InfoContext(ctx, "no pending version bump found for group", slog.String("group", group.Name))
				return nil
			}

			nextVersion, err := workspace.GetNextVersion(ctx, workspace.ExecRunner{}, res.Dir, group, status.Level)
			if err != nil {
				logger.ErrorContext(ctx, "failed to get next version", slog.String("group", group.Name), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fmt.Println(nextVersion)

			return nil
		},
	}
}
