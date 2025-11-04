package builtins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/tidwall/sjson"
	"github.com/urfave/cli/v3"
)

var (
	npmPackageFlag = &cli.StringFlag{
		Name:      "package",
		Usage:     "The path to a package.json file containing version information",
		Required:  true,
		TakesFile: true,
	}
	npmPackagesFlag = &cli.StringSliceFlag{
		Name:      "package",
		Usage:     "package.json paths to update (repeatable flag)",
		Required:  true,
		TakesFile: true,
	}
)

func npmPackagePath(c *cli.Command) string {
	return c.String("package")
}

func npmPackagePaths(c *cli.Command) []string {
	return c.StringSlice("package")
}

func newNPMCurrentCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "current:npm",
		Usage: "Get the current version of a release group using an npm package.json file",
		Flags: []cli.Flag{
			npmPackageFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			packageFile := npmPackagePath(c)
			data, err := os.ReadFile(packageFile)
			if err != nil {
				logger.ErrorContext(ctx, "failed to read package file", slog.String("file", packageFile), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			var pkg struct {
				Version string `json:"version"`
			}

			err = json.Unmarshal(data, &pkg)
			if err != nil {
				logger.ErrorContext(ctx, "failed to parse package file", slog.String("file", packageFile), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			raw := pkg.Version
			if raw == "" {
				raw = "0.0.0"
			}

			sv, err := semver.NewVersion(raw)
			if err != nil {
				logger.ErrorContext(ctx, "failed to parse version", slog.String("version", pkg.Version), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fmt.Print(sv.String())

			return nil
		},
	}
}

func newNPMNextCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "next:npm",
		Usage: "Set version of a release group in an npm package.json file",
		Flags: []cli.Flag{
			npmPackagesFlag,
			nextVersionFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			for _, packageFile := range npmPackagePaths(c) {
				data, err := os.ReadFile(packageFile)
				switch {
				case errors.Is(err, os.ErrNotExist):
					logger.WarnContext(ctx, "package file does not exist", slog.String("file", packageFile))
					data = []byte("{}")
				case err != nil:
					logger.ErrorContext(ctx, "failed to read package file", slog.String("file", packageFile), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				bs, err := sjson.SetBytes(data, "version", c.String("version"))
				if err != nil {
					logger.ErrorContext(ctx, "failed to set version in package file", slog.String("file", packageFile), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				err = os.WriteFile(packageFile, bs, 0644)
				if err != nil {
					logger.ErrorContext(ctx, "failed to write package file", slog.String("file", packageFile), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				logger.InfoContext(ctx, "updated package version", slog.String("file", packageFile), slog.String("version", c.String("version")))
			}

			return nil
		},
	}
}
