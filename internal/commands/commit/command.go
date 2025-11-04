package commit

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
	"github.com/urfave/cli/v3"
)

func NewCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "commit",
		Usage: "Commit pending version bumps",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
		},
		Action: func(ctx context.Context, c *cli.Command) error {
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

			repo, err := workspace.OpenGitRepository(dir)
			switch {
			case err != nil:
				logger.WarnContext(ctx, "failed to open git repository", slog.String("dir", dir), slog.String("error", err.Error()))
			case repo == nil:
				logger.WarnContext(ctx, "git repository not found", slog.String("dir", dir))
			}

			highestBump := make(map[string]workspace.BumpLevel)
			for _, g := range cfg.Groups {
				highestBump[g.Name] = workspace.BumpLevelNone
			}

			pattern := workspace.BumpFilename(dir, "*")
			matches, err := filepath.Glob(pattern)
			if err != nil {
				logger.ErrorContext(ctx, "failed to glob bump files", slog.String("dir", dir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			if len(matches) == 0 {
				logger.InfoContext(ctx, "no pending version bumps found", slog.String("pattern", pattern))
				return nil
			}

			type logEntry struct {
				timestamp int64
				content   string
			}

			type logs struct {
				major []logEntry
				minor []logEntry
				patch []logEntry
			}
			groupLogs := make(map[string]*logs)

			var itererr error
			lo.ForEachWhile(matches, func(match string, _ int) bool {
				f, err := os.Open(match)
				if err != nil {
					logger.ErrorContext(ctx, "failed to open bump file", slog.String("file", match), slog.String("error", err.Error()))
					itererr = err
					return false
				}
				defer f.Close()

				content, err := io.ReadAll(f)
				if err != nil {
					logger.ErrorContext(ctx, "failed to read bump file", slog.String("file", match), slog.String("error", err.Error()))
					itererr = err
					return false
				}

				frontMatter := make(map[string]string)
				message, err := extractFrontMatter(string(content), &frontMatter)
				if err != nil {
					logger.ErrorContext(ctx, "failed to extract front matter", slog.String("file", match), slog.String("error", err.Error()))
					itererr = err
					return false
				}

				timestamp := int64(0)
				commit, err := workspace.GetFirstCommitSHA(repo, match)
				switch {
				case err != nil:
					logger.WarnContext(ctx, "failed to get initial commit SHA for bump file", slog.String("error", err.Error()), slog.String("file", match))
				case commit == nil:
					logger.WarnContext(ctx, "initial commit SHA for bump file not found", slog.String("file", match))
				default:
					timestamp = commit.When.Unix()
					message = fmt.Sprintf("%s: %s", commit.SHA, message)
				}

				entry := logEntry{timestamp: timestamp, content: message}

				for groupName, level := range frontMatter {
					if _, ok := highestBump[groupName]; !ok {
						logger.WarnContext(ctx, "skipping bump for unknown group", slog.String("file", match), slog.String("group", groupName))
						continue
					}

					if _, ok := groupLogs[groupName]; !ok {
						groupLogs[groupName] = &logs{
							major: []logEntry{},
							minor: []logEntry{},
							patch: []logEntry{},
						}
					}

					switch level {
					case "major":
						highestBump[groupName] = max(highestBump[groupName], workspace.BumpLevelMajor)
						groupLogs[groupName].major = append(groupLogs[groupName].major, entry)
					case "minor":
						highestBump[groupName] = max(highestBump[groupName], workspace.BumpLevelMinor)
						groupLogs[groupName].minor = append(groupLogs[groupName].minor, entry)
					case "patch":
						highestBump[groupName] = max(highestBump[groupName], workspace.BumpLevelPatch)
						groupLogs[groupName].patch = append(groupLogs[groupName].patch, entry)
					default:
						logger.WarnContext(ctx, "unknown level in bump file front matter", slog.String("file", match), slog.String("group", groupName), slog.String("level", level))
					}
				}

				return true
			})
			if itererr != nil {
				return cmd.Failed(itererr)
			}

			lo.ForEachWhile(matches, func(match string, _ int) bool {
				err := os.Remove(match)
				if err != nil {
					logger.ErrorContext(ctx, "failed to remove bump file", slog.String("file", match), slog.String("error", err.Error()))
					itererr = err
					return false
				}
				return true
			})
			if itererr != nil {
				return cmd.Failed(itererr)
			}

			for groupName, level := range highestBump {
				g, ok := cfgGroups[groupName]
				if !ok {
					logger.WarnContext(ctx, "skipping commit for unknown group", slog.String("group", groupName))
					continue
				}

				if level == 0 {
					continue
				}

				gl, ok := groupLogs[groupName]
				if !ok {
					gl = &logs{
						major: []logEntry{},
						minor: []logEntry{},
						patch: []logEntry{},
					}
				}

				slices.SortStableFunc(gl.major, func(a, b logEntry) int {
					return cmp.Compare(a.timestamp, b.timestamp)
				})
				slices.SortStableFunc(gl.minor, func(a, b logEntry) int {
					return cmp.Compare(a.timestamp, b.timestamp)
				})
				slices.SortStableFunc(gl.patch, func(a, b logEntry) int {
					return cmp.Compare(a.timestamp, b.timestamp)
				})

				amendFlags := make([]string, 0, len(gl.major)+len(gl.minor)+len(gl.patch)+2)
				amendFlags = append(amendFlags, "--group", groupName)

				for _, entry := range gl.major {
					amendFlags = append(amendFlags, "--major", entry.content)
				}
				for _, entry := range gl.minor {
					amendFlags = append(amendFlags, "--minor", entry.content)
				}
				for _, entry := range gl.patch {
					amendFlags = append(amendFlags, "--patch", entry.content)
				}

				nextVersion, err := getNextVersion(ctx, g, level)
				if err != nil {
					logger.ErrorContext(ctx, "failed to get next version", slog.String("group", groupName), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				err = commitVersionBump(ctx, g, nextVersion)
				if err != nil {
					logger.ErrorContext(ctx, "failed to commit version bump", slog.String("group", groupName), slog.String("version", nextVersion), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				err = commitChangelog(ctx, g, nextVersion, amendFlags)
				if err != nil {
					logger.ErrorContext(ctx, "failed to commit changelog", slog.String("group", groupName), slog.String("version", nextVersion), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}
			}

			return nil
		},
	}
}

func getNextVersion(ctx context.Context, group workspace.ReleaseGroup, level workspace.BumpLevel) (string, error) {
	if len(group.CurrentCMD) == 0 {
		return "", errors.New("no current version command defined for release group")
	}

	currentProg := group.CurrentCMD[0]
	currentArgs := group.CurrentCMD[1:]
	cmd := exec.CommandContext(ctx, currentProg, currentArgs...)
	stdout := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("BUMPER_GROUP=%s", group.Name),
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("execute current version command: %w", err)
	}

	currentVersionStr := strings.TrimSpace(stdout.String())
	currentSemver, err := semver.NewVersion(currentVersionStr)
	if err != nil {
		return "", fmt.Errorf("%s: parse current version string: %w", currentVersionStr, err)
	}

	switch level {
	case workspace.BumpLevelMajor:
		return currentSemver.IncMajor().String(), nil
	case workspace.BumpLevelMinor:
		return currentSemver.IncMinor().String(), nil
	case workspace.BumpLevelPatch:
		return currentSemver.IncPatch().String(), nil
	default:
		return "", errors.New("invalid bump level for next version")
	}
}

func commitVersionBump(ctx context.Context, group workspace.ReleaseGroup, versionStr string) error {
	if len(group.NextCMD) == 0 {
		return errors.New("no next version command defined for release group")
	}

	nextProg := group.NextCMD[0]
	nextArgs := group.NextCMD[1:]
	cmd := exec.CommandContext(ctx, nextProg, nextArgs...)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("BUMPER_GROUP=%s", group.Name),
		fmt.Sprintf("BUMPER_GROUP_NEXT_VERSION=%s", versionStr),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("execute next version command: %w", err)
	}

	return nil
}

func commitChangelog(ctx context.Context, group workspace.ReleaseGroup, versionStr string, flags []string) error {
	if len(group.ChangelogCMD) == 0 {
		return errors.New("no changelog command defined for release group")
	}

	changelogProg := group.ChangelogCMD[0]
	changelogArgs := append(group.ChangelogCMD[1:], flags...)
	cmd := exec.CommandContext(ctx, changelogProg, changelogArgs...)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("BUMPER_GROUP=%s", group.Name),
		fmt.Sprintf("BUMPER_GROUP_NEXT_VERSION=%s", versionStr),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("execute amend changelog command: %w", err)
	}

	return nil
}

func extractFrontMatter(content string, dst any) (string, error) {
	state := "initial"
	fm := ""
	rest := ""
	for line := range strings.Lines(content) {
		switch {
		case state == "initial":
			if line != "---\n" {
				return "", errors.New("front matter must start with ---")
			}
			state = "frontmatter"
		case state == "frontmatter" && line == "---\n":
			state = "slurping"
		case state == "frontmatter":
			fm += line
		case state == "slurping":
			rest += line
		default:
			return "", errors.New("invalid front matter parse state")
		}
	}

	fm = strings.TrimSpace(fm)
	if fm == "" {
		fm = "{}"
	}

	if err := yaml.Unmarshal([]byte(fm), dst); err != nil {
		return "", fmt.Errorf("parse frontmatter yaml: %w", err)
	}

	return strings.TrimSpace(rest), nil
}
