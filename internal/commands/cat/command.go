package cat

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

func NewCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "cat",
		Usage: "Show release notes for a given version",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			&cli.StringFlag{
				Name:    "group",
				Usage:   "The group to show release notes for",
				Sources: cli.EnvVars("BUMPER_GROUP"),
			},
			&cli.StringFlag{
				Name:     "version",
				Usage:    "The version to show release notes for",
				Required: true,
				Sources:  cli.EnvVars("BUMPER_GROUP_VERSION"),
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

			inv, err := workspace.NewCatInvocation(group, c.String("version"))
			if err != nil {
				logger.ErrorContext(ctx, "no cat command defined for group", slog.String("group", group.Name))
				return cmd.Failed(err)
			}

			runner := workspace.ExecRunner{}
			if err := runner.Run(ctx, res.Dir, inv, os.Stdout); err != nil {
				logger.ErrorContext(ctx, "failed to execute cat command", slog.String("error", err.Error()), slog.String("command", strings.Join(group.CatCMD, " ")))
				return cmd.Failed(err)
			}

			return nil
		},
	}
}
