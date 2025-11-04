package cat

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
		Name:  "cat",
		Usage: "Show release notes for a given version",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			&cli.StringFlag{
				Name:     "group",
				Usage:    "The group to show release notes for",
				Required: true,
				Sources:  cli.EnvVars("BUMPER_GROUP"),
			},
			&cli.StringFlag{
				Name:     "version",
				Usage:    "The version to show release notes for",
				Required: true,
				Sources:  cli.EnvVars("BUMPER_GROUP_VERSION"),
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

			if len(group.CatCMD) == 0 {
				logger.ErrorContext(ctx, "no cat command defined for group", slog.String("group", groupName))
				return cmd.Failed(err)
			}

			program := group.CatCMD[0]
			args := group.CatCMD[1:]
			catcmd := exec.CommandContext(ctx, program, args...)
			catcmd.Env = append(
				os.Environ(),
				fmt.Sprintf("BUMPER_GROUP=%s", groupName),
				fmt.Sprintf("BUMPER_GROUP_VERSION=%s", c.String("version")),
			)
			catcmd.Stdout = os.Stdout
			catcmd.Stderr = os.Stderr

			if err := catcmd.Run(); err != nil {
				logger.ErrorContext(ctx, "failed to execute cat command", slog.String("error", err.Error()), slog.String("command", strings.Join(group.CatCMD, " ")))
				return cmd.Failed(err)
			}

			return nil
		},
	}
}
