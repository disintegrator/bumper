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
			res, err := shared.Resolve(ctx, logger, shared.DirFlag(c))
			if err != nil {
				return err
			}

			group, err := res.Group(ctx, logger, c.String("group"))
			if err != nil {
				return err
			}

			currentVersion, err := workspace.GetCurrentVersion(ctx, workspace.ExecRunner{}, res.Dir, group)
			if err != nil {
				logger.ErrorContext(ctx, "failed to get current version", slog.String("group", group.Name), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fmt.Println(currentVersion)

			return nil
		},
	}
}
