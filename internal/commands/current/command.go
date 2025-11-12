package current

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

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

			if len(group.CurrentCMD) == 0 {
				logger.ErrorContext(ctx, "no current command defined for group", slog.String("group", groupName))
				return cmd.Failed(err)
			}

			program := group.CurrentCMD[0]
			args := group.CurrentCMD[1:]
			currentcmd := exec.CommandContext(ctx, program, args...)
			currentcmd.Dir = dir
			currentcmd.Env = append(
				os.Environ(),
				fmt.Sprintf("BUMPER_GROUP=%s", groupName),
			)
			currentcmd.Stdout = os.Stdout
			currentcmd.Stderr = os.Stderr

			if err := currentcmd.Run(); err != nil {
				logger.ErrorContext(ctx, "failed to execute current command", slog.String("error", err.Error()), slog.String("command", strings.Join(group.CurrentCMD, " ")))
				return cmd.Failed(err)
			}

			return nil
		},
	}
}
