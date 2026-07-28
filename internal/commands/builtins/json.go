package builtins

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/urfave/cli/v3"
)

func newJSONCurrentCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "current:json",
		Usage: "Get the current version from a key in a JSON file",
		Flags: []cli.Flag{
			filePathFlag,
			keyFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			path := filePath(c)
			key := keyPath(c)

			data, err := os.ReadFile(path)
			if err != nil {
				logger.ErrorContext(ctx, "failed to read json file", slog.String("file", path), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			result := gjson.GetBytes(data, key)
			if !result.Exists() {
				err := fmt.Errorf("key %q not found in %s", key, path)
				logger.ErrorContext(ctx, "failed to find version key", slog.String("file", path), slog.String("key", key))
				return cmd.Failed(err)
			}

			sv, err := semver.NewVersion(result.String())
			if err != nil {
				logger.ErrorContext(ctx, "failed to parse version", slog.String("version", result.String()), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fmt.Print(sv.String())

			return nil
		},
	}
}

func newJSONNextCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "next:json",
		Usage: "Set the next version at a key in a JSON file",
		Flags: []cli.Flag{
			filePathsFlag,
			keyFlag,
			nextVersionFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			key := keyPath(c)
			version := nextVersion(c)

			for _, path := range filePaths(c) {
				data, err := os.ReadFile(path)
				if err != nil {
					logger.ErrorContext(ctx, "failed to read json file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				if !gjson.GetBytes(data, key).Exists() {
					err := fmt.Errorf("key %q not found in %s", key, path)
					logger.ErrorContext(ctx, "failed to find version key", slog.String("file", path), slog.String("key", key))
					return cmd.Failed(err)
				}

				bs, err := sjson.SetBytes(data, key, version)
				if err != nil {
					logger.ErrorContext(ctx, "failed to set version in json file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				err = os.WriteFile(path, bs, 0644)
				if err != nil {
					logger.ErrorContext(ctx, "failed to write json file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				logger.InfoContext(ctx, "updated version", slog.String("file", path), slog.String("key", key), slog.String("version", version))
			}

			return nil
		},
	}
}
