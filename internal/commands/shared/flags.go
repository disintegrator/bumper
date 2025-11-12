package shared

import (
	"context"
	"errors"
	"log/slog"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

func NewDirFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:  "dir",
		Usage: "Working directory which contains the .bumper directory",
	}
}

func DirFlag(c *cli.Command) string {
	return c.String("dir")
}

func GroupFlagOrDefault(ctx context.Context, logger *slog.Logger, c *cli.Command, cfg *workspace.Config) (string, error) {
	groupName := c.String("group")
	if groupName == "" && len(cfg.Groups) == 1 {
		groupName = cfg.Groups[0].Name
	}

	if groupName == "" {
		err := errors.New("--group flag is required")
		if len(cfg.Groups) > 0 {
			err = errors.New("--group flag is required when multiple release groups are defined")
		}

		logger.ErrorContext(ctx, err.Error())
		return "", cmd.Failed(err)
	}

	return groupName, nil
}
