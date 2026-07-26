package builtins

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/disintegrator/bumper/internal/changelog"
	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

// newChangelogPathFlag returns the --path flag shared by the default
// changelog commands. It carries no static default: both commands fall back
// to CHANGELOG.md in the workspace root, which is only known after the
// workspace is resolved.
func newChangelogPathFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:      "path",
		Usage:     "The path to the changelog file (defaults to CHANGELOG.md in the workspace root)",
		Sources:   cli.EnvVars("BUMPER_CHANGELOG_PATH"),
		TakesFile: true,
	}
}

func changelogPath(c *cli.Command, workspaceDir string) string {
	if path := c.String("path"); path != "" {
		return path
	}
	return filepath.Join(workspaceDir, "CHANGELOG.md")
}

func newDefaultAmendChangelogCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:                      "amendlog:default",
		Usage:                     "Get the current version of a release group using the default strategy",
		DisableSliceFlagSeparator: true,
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			newChangelogPathFlag(),
			releaseGroupFlag,
			nextVersionFlag,
			&cli.StringSliceFlag{
				Name:  "major",
				Usage: "Major changes in the given version (repeatable flag)",
			},
			&cli.StringSliceFlag{
				Name:  "minor",
				Usage: "Minor changes in the given version (repeatable flag)",
			},
			&cli.StringSliceFlag{
				Name:  "patch",
				Usage: "Patch changes in the given version (repeatable flag)",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			groupName := releaseGroup(c)
			res, err := shared.Resolve(ctx, logger, shared.DirFlag(c))
			if err != nil {
				return err
			}

			displayName := groupName
			group, ok := res.Config.IndexReleaseGroups()[groupName]
			if ok {
				if group.DisplayName != "" {
					displayName = group.DisplayName
				}
			} else {
				logger.WarnContext(ctx, "release group not in config", slog.String("group", groupName), slog.String("config", workspace.ConfigFilename(res.Dir)))
			}

			filename := changelogPath(c, res.Dir)
			release := changelog.Release{
				DisplayName: displayName,
				Version:     nextVersion(c),
				Major:       c.StringSlice("major"),
				Minor:       c.StringSlice("minor"),
				Patch:       c.StringSlice("patch"),
			}

			if err := amendChangelogFile(filename, release); err != nil {
				logger.ErrorContext(ctx, "failed to amend changelog", slog.String("file", filename), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			return nil
		},
	}
}

// amendChangelogFile opens the changelog for the stream-based amendment,
// treating a missing file as an empty changelog.
func amendChangelogFile(filename string, release changelog.Release) error {
	content, err := os.ReadFile(filename)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read changelog file: %w", err)
	}

	var amended bytes.Buffer
	if err := changelog.Amend(&amended, bytes.NewReader(content), release); err != nil {
		return err
	}

	if err := os.WriteFile(filename, amended.Bytes(), 0644); err != nil {
		return fmt.Errorf("write changelog file: %w", err)
	}

	return nil
}

func newDefaultCatChangelogCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "cat:default",
		Usage: "Get the release notes of a release group using the default strategy",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			newChangelogPathFlag(),
			releaseGroupFlag,
			versionFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			groupName := releaseGroup(c)
			versionStr := version(c)
			res, err := shared.Resolve(ctx, logger, shared.DirFlag(c))
			if err != nil {
				return err
			}

			group, err := res.Group(ctx, logger, groupName)
			if err != nil {
				return err
			}

			displayName := group.Name
			if group.DisplayName != "" {
				displayName = group.DisplayName
			}

			logfile := changelogPath(c, res.Dir)
			file, err := os.Open(logfile)
			if err != nil {
				logger.ErrorContext(ctx, "failed to open changelog", slog.String("file", logfile), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}
			defer file.Close()

			result, err := changelog.Section(file, displayName, versionStr)
			if err != nil {
				logger.ErrorContext(ctx, "failed to get changelog for group", slog.String("group", groupName), slog.String("version", versionStr), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}
			if result == "" {
				logger.ErrorContext(ctx, "no release notes found for version", slog.String("version", versionStr), slog.String("group", groupName))
				return cmd.Failed(fmt.Errorf("no release notes found for version %s in group %s", versionStr, groupName))
			}

			fmt.Println(result)

			return nil
		},
	}
}
