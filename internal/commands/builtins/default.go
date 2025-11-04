package builtins

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/semver/v3"
	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

func newDefaultCurrentCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "current:default",
		Usage: "Get the current version of a release group using the default strategy",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			releaseGroupFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			rawdir := shared.DirFlag(c)
			dir, err := workspace.GetWd(rawdir)
			if err != nil {
				logger.ErrorContext(ctx, "workspace directory not found", slog.String("dir", rawdir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			var versions struct {
				Values map[string]string `toml:"versions,omitempty,omitzero"`
			}

			versionFile := workspace.VersionFilename(dir)
			_, err = toml.DecodeFile(versionFile, &versions)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				logger.ErrorContext(ctx, "failed to read versions file", slog.String("file", versionFile), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			v, ok := versions.Values[releaseGroup(c)]
			if !ok {
				v = "0.0.0"
			}

			sv, err := semver.NewVersion(v)
			if err != nil {
				logger.ErrorContext(ctx, "failed to parse version", slog.String("version", v), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fmt.Print(sv.String())

			return nil
		},
	}
}

func newDefaultNextCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "next:default",
		Usage: "Set the version of a release group using the default strategy",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			releaseGroupFlag,
			nextVersionFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			rawdir := shared.DirFlag(c)
			dir, err := workspace.GetWd(rawdir)
			if err != nil {
				logger.ErrorContext(ctx, "failed to initialize project", slog.String("dir", rawdir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			var versions struct {
				Values map[string]string `toml:"versions,omitempty,omitzero"`
			}

			versionFile := workspace.VersionFilename(dir)
			_, err = toml.DecodeFile(versionFile, &versions)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				logger.ErrorContext(ctx, "failed to read versions file", slog.String("file", versionFile), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			if versions.Values == nil {
				versions.Values = make(map[string]string)
			}
			versions.Values[releaseGroup(c)] = c.String("version")

			f, err := os.Create(versionFile)
			if err != nil {
				logger.ErrorContext(ctx, "failed to open versions file for writing", slog.String("file", versionFile), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}
			defer f.Close()

			encoder := toml.NewEncoder(f)
			err = encoder.Encode(versions)
			if err != nil {
				logger.ErrorContext(ctx, "failed to write versions file", slog.String("file", versionFile), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			logger.InfoContext(ctx, "updated version", slog.String("group", c.String("group")), slog.String("version", c.String("version")))

			return nil
		},
	}
}
