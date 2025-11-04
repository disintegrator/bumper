package initialize

import (
	"context"
	"log/slog"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

func NewCommand(logger *slog.Logger) *cli.Command {
	dirFlag := shared.NewDirFlag()
	dirFlag.Value = "."

	return &cli.Command{
		Name:  "init",
		Usage: "Initialize a new project",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			dir := shared.DirFlag(c)

			err := workspace.Initialize(dir)
			if err != nil {
				logger.ErrorContext(ctx, "failed to initialize project", slog.String("dir", dir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			return nil
		},
	}
}
