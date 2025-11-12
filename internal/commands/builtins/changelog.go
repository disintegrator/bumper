package builtins

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

func newDefaultAmendChangelogCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:                      "amendlog:default",
		Usage:                     "Get the current version of a release group using the default strategy",
		DisableSliceFlagSeparator: true,
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			&cli.StringFlag{
				Name:      "path",
				Usage:     "The path to the changelog file",
				Sources:   cli.EnvVars("BUMPER_CHANGELOG_PATH"),
				TakesFile: true,
			},
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
			versionStr := nextVersion(c)
			major := c.StringSlice("major")
			minor := c.StringSlice("minor")
			patch := c.StringSlice("patch")
			rawdir := shared.DirFlag(c)
			dir, err := workspace.GetWd(rawdir)
			if err != nil {
				logger.ErrorContext(ctx, "workspace directory not found", slog.String("dir", rawdir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			cfg, err := shared.LoadConfig(ctx, logger, dir)
			if err != nil {
				return err
			}

			displayName := groupName
			groups := cfg.IndexReleaseGroups()
			group, ok := groups[groupName]
			if ok {
				if group.DisplayName != "" {
					displayName = group.DisplayName
				}
			} else {
				logger.WarnContext(ctx, "release group not in config", slog.String("group", groupName), slog.String("config", workspace.ConfigFilename(dir)))
			}

			filename := c.String("path")
			if filename == "" {
				filename = filepath.Join(dir, "CHANGELOG.md")
			}

			err = amendChangelog(filename, displayName, versionStr, major, minor, patch)
			if err != nil {
				logger.ErrorContext(ctx, "failed to amend changelog", slog.String("file", filename), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			return nil
		},
	}
}

func amendChangelog(
	filename string,
	displayName string,
	versionStr string,
	major []string,
	minor []string,
	patch []string,
) error {
	insertion := fmt.Sprintf("## %s %s\n\n", displayName, versionStr)
	if len(major) > 0 {
		insertion += "### Major Changes\n\n"
		for _, entry := range major {
			insertion += formatEntry(entry)
		}
		insertion += "\n"
	}
	if len(minor) > 0 {
		insertion += "### Minor Changes\n\n"
		for _, entry := range minor {
			insertion += formatEntry(entry)
		}
		insertion += "\n"
	}
	if len(patch) > 0 {
		insertion += "### Patch Changes\n\n"
		for _, entry := range patch {
			insertion += formatEntry(entry)
		}
		insertion += "\n"
	}

	insertion = strings.TrimSpace(insertion)

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("open changelog file: %w", err)
	}
	defer file.Close()

	// Check if file is empty and initialize if needed
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat changelog file: %w", err)
	}

	if info.Size() == 0 {
		if _, err := file.WriteString("# Changelog"); err != nil {
			return fmt.Errorf("initialize changelog file: %w", err)
		}
		if _, err := file.Seek(0, 0); err != nil {
			return fmt.Errorf("reset initial changelog reader: %w", err)
		}
	}

	scanner := bufio.NewScanner(file)
	var lines []string
	inserted := false

	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)

		if !inserted && strings.HasPrefix(line, "# Changelog") {
			lines = append(lines, "", insertion)
			inserted = true
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read changelog file: %w", err)
	}

	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("truncate changelog file: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek to start: %w", err)
	}

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("write changelog file: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush changelog file: %w", err)
	}

	return nil
}

func formatEntry(entry string) string {
	if entry == "" {
		return ""
	}

	entry = "- " + strings.ReplaceAll(entry, "\n", "\n  ")
	entry = regexp.MustCompile(`(?m)^\s+$`).ReplaceAllString(entry, "")

	return entry + "\n"
}

func newDefaultCatChangelogCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "cat:default",
		Usage: "Get the release notes of a release group using the default strategy",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			&cli.StringFlag{
				Name:      "path",
				Usage:     "The path to the changelog file",
				Value:     "CHANGELOG.md",
				Sources:   cli.EnvVars("BUMPER_CHANGELOG_PATH"),
				TakesFile: true,
			},
			releaseGroupFlag,
			versionFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			logfile := c.String("path")
			groupName := releaseGroup(c)
			versionStr := version(c)
			rawdir := shared.DirFlag(c)
			dir, err := workspace.GetWd(rawdir)
			if err != nil {
				logger.ErrorContext(ctx, "workspace directory not found", slog.String("dir", rawdir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			cfg, err := shared.LoadConfig(ctx, logger, dir)
			if err != nil {
				return err
			}

			group, ok := cfg.IndexReleaseGroups()[groupName]
			if !ok {
				logger.ErrorContext(ctx, "release group not found", slog.String("group", groupName))
				return cmd.Failed(err)
			}

			result, err := changelogForGroup(group, logfile, versionStr)
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

func changelogForGroup(group workspace.ReleaseGroup, logfile string, versionStr string) (string, error) {
	displayName := group.Name
	if group.DisplayName != "" {
		displayName = group.DisplayName
	}

	file, err := os.Open(logfile)
	if err != nil {
		return "", fmt.Errorf("open changelog: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	state := "search"
	var output strings.Builder

scanloop:
	for scanner.Scan() {
		line := scanner.Text()

		switch state {
		case "search":
			if strings.HasPrefix(line, fmt.Sprintf("## %s %s", displayName, versionStr)) {
				state = "collect"
				output.WriteString(line + "\n")
			}
		case "collect":
			if strings.HasPrefix(line, "## ") {
				break scanloop
			} else {
				output.WriteString(line + "\n")
			}
		default:
			panic("unreachable state")
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read changelog: %w", err)
	}

	return strings.TrimSpace(output.String()), nil
}
