package workspace

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
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
)

type LogEntry struct {
	Timestamp int64
	Content   string
}

type ReleaseGroupStatus struct {
	Level     BumpLevel
	MajorLogs []LogEntry
	MinorLogs []LogEntry
	PatchLogs []LogEntry
}

func CollectBumps(ctx context.Context, logger *slog.Logger, dir string, cfg *Config) (map[string]*ReleaseGroupStatus, error) {
	statuses := make(map[string]*ReleaseGroupStatus)

	repo, err := openGitRepository(dir)
	switch {
	case errors.Is(err, errNoGitRepository):
		logger.WarnContext(ctx, "git repository not found", slog.String("dir", dir))
	case err != nil:
		logger.WarnContext(ctx, "failed to open git repository", slog.String("dir", dir), slog.String("error", err.Error()))
	}

	highestBump := make(map[string]BumpLevel)
	for _, g := range cfg.Groups {
		highestBump[g.Name] = BumpLevelNone
	}

	pattern := BumpFilename(dir, "*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob bump files: %w", err)
	}

	if len(matches) == 0 {
		return statuses, nil
	}

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
		var commit *vcsCommit
		if repo != nil {
			commit, err = getFirstCommit(repo, match)
		}
		switch {
		case err != nil:
			logger.WarnContext(ctx, "failed to get initial commit SHA for bump file", slog.String("error", err.Error()), slog.String("file", match))
		case commit == nil:
			if repo != nil {
				logger.WarnContext(ctx, "initial commit SHA for bump file not found", slog.String("file", match))
			}
		default:
			timestamp = commit.When.Unix()
			message = fmt.Sprintf("%s: %s", commit.SHA, message)
		}

		entry := LogEntry{Timestamp: timestamp, Content: message}

		for groupName, level := range frontMatter {
			if _, ok := highestBump[groupName]; !ok {
				logger.WarnContext(ctx, "skipping bump for unknown group", slog.String("file", match), slog.String("group", groupName))
				continue
			}

			if _, ok := statuses[groupName]; !ok {
				statuses[groupName] = &ReleaseGroupStatus{
					Level:     BumpLevelNone,
					MajorLogs: []LogEntry{},
					MinorLogs: []LogEntry{},
					PatchLogs: []LogEntry{},
				}
			}

			switch level {
			case "major":
				statuses[groupName].Level = max(statuses[groupName].Level, BumpLevelMajor)
				statuses[groupName].MajorLogs = append(statuses[groupName].MajorLogs, entry)
			case "minor":
				statuses[groupName].Level = max(statuses[groupName].Level, BumpLevelMinor)
				statuses[groupName].MinorLogs = append(statuses[groupName].MinorLogs, entry)
			case "patch":
				statuses[groupName].Level = max(statuses[groupName].Level, BumpLevelPatch)
				statuses[groupName].PatchLogs = append(statuses[groupName].PatchLogs, entry)
			default:
				logger.WarnContext(ctx, "unknown level in bump file front matter", slog.String("file", match), slog.String("group", groupName), slog.String("level", level))
			}
		}

		return true
	})
	if itererr != nil {
		return nil, fmt.Errorf("process bump files: %w", itererr)
	}

	for _, status := range statuses {
		slices.SortStableFunc(status.MajorLogs, func(a, b LogEntry) int {
			return cmp.Compare(a.Timestamp, b.Timestamp)
		})
		slices.SortStableFunc(status.MinorLogs, func(a, b LogEntry) int {
			return cmp.Compare(a.Timestamp, b.Timestamp)
		})
		slices.SortStableFunc(status.PatchLogs, func(a, b LogEntry) int {
			return cmp.Compare(a.Timestamp, b.Timestamp)
		})
	}

	return statuses, nil
}

func DeleteBumps(ctx context.Context, dir string) error {
	pattern := BumpFilename(dir, "*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob bump files: %w", err)
	}

	if len(matches) == 0 {
		return nil
	}

	for _, match := range matches {
		err := os.Remove(match)
		if err != nil {
			return fmt.Errorf("remove bump file %s: %w", match, err)
		}
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

func GetCurrentVersion(ctx context.Context, dir string, group ReleaseGroup) (*semver.Version, error) {
	if len(group.CurrentCMD) == 0 {
		return nil, errors.New("no current version command defined for release group")
	}

	currentProg := group.CurrentCMD[0]
	currentArgs := group.CurrentCMD[1:]
	cmd := exec.CommandContext(ctx, currentProg, currentArgs...)
	stdout := new(bytes.Buffer)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("BUMPER_GROUP=%s", group.Name),
	)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("execute current version command: %w", err)
	}

	currentVersionStr := strings.TrimSpace(stdout.String())
	currentSemver, err := semver.NewVersion(currentVersionStr)
	if err != nil {
		return nil, fmt.Errorf("%s: parse current version string: %w", currentVersionStr, err)
	}

	return currentSemver, nil
}

func GetNextVersion(ctx context.Context, dir string, group ReleaseGroup, level BumpLevel) (string, error) {
	if len(group.CurrentCMD) == 0 {
		return "", errors.New("no current version command defined for release group")
	}

	currentProg := group.CurrentCMD[0]
	currentArgs := group.CurrentCMD[1:]
	cmd := exec.CommandContext(ctx, currentProg, currentArgs...)
	stdout := new(bytes.Buffer)
	cmd.Dir = dir
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
	case BumpLevelMajor:
		return currentSemver.IncMajor().String(), nil
	case BumpLevelMinor:
		return currentSemver.IncMinor().String(), nil
	case BumpLevelPatch:
		return currentSemver.IncPatch().String(), nil
	default:
		return "", errors.New("invalid bump level for next version")
	}
}
