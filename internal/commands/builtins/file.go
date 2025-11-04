package builtins

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/urfave/cli/v3"
)

var (
	filePathFlag = &cli.StringFlag{
		Name:      "path",
		Usage:     "Path to a file containing the version",
		Required:  true,
		TakesFile: true,
	}

	filePathsFlag = &cli.StringSliceFlag{
		Name:      "path",
		Usage:     "Paths to files containing the version (repeatable flag)",
		Required:  true,
		TakesFile: true,
	}
)

func filePath(c *cli.Command) string {
	return c.String("path")
}

func filePaths(c *cli.Command) []string {
	return c.StringSlice("path")
}

func newFileCurrentCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "current:file",
		Usage: "Get the current version from a simple text file",
		Flags: []cli.Flag{
			filePathFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			path := filePath(c)
			data, err := os.ReadFile(path)
			switch {
			case errors.Is(err, os.ErrNotExist):
				logger.WarnContext(ctx, "version file does not exist", slog.String("file", path))
				data = []byte("0.0.0")
			case err != nil:
				logger.ErrorContext(ctx, "failed to read version file", slog.String("file", path), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			versionStr := strings.TrimSpace(string(data))
			sv, err := semver.NewVersion(versionStr)
			if err != nil {
				logger.ErrorContext(ctx, "failed to parse version", slog.String("version", versionStr), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fmt.Print(sv.String())

			return nil
		},
	}
}

func newFileNextCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "next:file",
		Usage: "Set the next version in a simple text file",
		Flags: []cli.Flag{
			filePathsFlag,
			nextVersionFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			for _, path := range filePaths(c) {
				err := os.WriteFile(path, []byte(strings.TrimSpace(c.String("version"))), 0644)
				if err != nil {
					logger.ErrorContext(ctx, "failed to write version file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				logger.InfoContext(ctx, "updated version file", slog.String("file", path), slog.String("version", c.String("version")))
			}
			return nil
		},
	}
}
