package workspace

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
)

type LogEntry struct {
	Timestamp int64
	Commit    string
	Content   string
}

type ReleaseGroupStatus struct {
	Level     BumpLevel
	MajorLogs []LogEntry
	MinorLogs []LogEntry
	PatchLogs []LogEntry
}

// ParsedBump is one bump file's parsed content plus its provenance: the
// levels recorded per release group, the changelog message, and the commit
// that introduced the file (nil when unresolved).
type ParsedBump struct {
	File    string
	Levels  map[string]string
	Message string
	Commit  *Commit
}

// CollectBumps composes GatherBumps and SquashBumps: it reads the pending
// bump files in dir and reduces them to per-group release status.
func CollectBumps(ctx context.Context, logger *slog.Logger, dir string, cfg *Config, provenance Provenance) (map[string]*ReleaseGroupStatus, error) {
	bumps, err := GatherBumps(ctx, logger, dir, provenance)
	if err != nil {
		return nil, err
	}

	return SquashBumps(ctx, logger, bumps, cfg), nil
}

// GatherBumps globs the pending bump files in dir, parses each file's front
// matter and message, and resolves the commit that introduced it.
func GatherBumps(ctx context.Context, logger *slog.Logger, dir string, provenance Provenance) ([]ParsedBump, error) {
	pattern := BumpFilename(dir, "*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob bump files: %w", err)
	}

	if len(matches) == 0 {
		return nil, nil
	}

	gitInfo, err := provenance.Resolve(ctx, matches)
	if err != nil {
		return nil, fmt.Errorf("resolve git info for bumps: %w", err)
	}

	bumps := make([]ParsedBump, 0, len(matches))
	for _, match := range matches {
		content, err := os.ReadFile(match)
		if err != nil {
			logger.ErrorContext(ctx, "failed to read bump file", slog.String("file", match), slog.String("error", err.Error()))
			return nil, fmt.Errorf("process bump files: %w", err)
		}

		levels := make(map[string]string)
		message, err := extractFrontMatter(string(content), &levels)
		if err != nil {
			logger.ErrorContext(ctx, "failed to extract front matter", slog.String("file", match), slog.String("error", err.Error()))
			return nil, fmt.Errorf("process bump files: %w", err)
		}

		bump := ParsedBump{File: match, Levels: levels, Message: message}
		if commit, ok := gitInfo[match]; ok {
			bump.Commit = &commit
		}

		bumps = append(bumps, bump)
	}

	return bumps, nil
}

// SquashBumps reduces parsed bump files to per-group release status: the
// highest bump level wins per group and changelog entries are ordered by
// commit timestamp. Bumps for groups absent from cfg and entries with unknown
// levels are skipped with a warning. It touches neither disk nor git.
func SquashBumps(ctx context.Context, logger *slog.Logger, bumps []ParsedBump, cfg *Config) map[string]*ReleaseGroupStatus {
	statuses := make(map[string]*ReleaseGroupStatus)

	knownGroups := cfg.IndexReleaseGroups()

	for _, bump := range bumps {
		entry := LogEntry{Content: bump.Message, Timestamp: 0, Commit: ""}
		if bump.Commit != nil {
			entry.Timestamp = bump.Commit.When.UnixNano()
			entry.Commit = bump.Commit.SHA[:min(7, len(bump.Commit.SHA))]
			entry.Content = fmt.Sprintf("%s: %s", entry.Commit, entry.Content)
		}

		for groupName, level := range bump.Levels {
			if _, ok := knownGroups[groupName]; !ok {
				logger.WarnContext(ctx, "skipping bump for unknown group", slog.String("file", bump.File), slog.String("group", groupName))
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
				logger.WarnContext(ctx, "unknown level in bump file front matter", slog.String("file", bump.File), slog.String("group", groupName), slog.String("level", level))
			}
		}
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

	return statuses
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

// extractFrontMatter splits a bump file into its YAML front matter (decoded
// into dst) and the markdown message that follows. It accepts both LF and
// CRLF line endings and files without a trailing newline.
func extractFrontMatter(content string, dst any) (string, error) {
	state := "initial"
	fm := ""
	var rest strings.Builder
	for line := range strings.Lines(content) {
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case state == "initial":
			if trimmed != "---" {
				return "", errors.New("front matter must start with ---")
			}
			state = "frontmatter"
		case state == "frontmatter" && trimmed == "---":
			state = "slurping"
		case state == "frontmatter":
			fm += trimmed + "\n"
		case state == "slurping":
			rest.WriteString(trimmed)
			rest.WriteByte('\n')
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

	return strings.TrimSpace(rest.String()), nil
}

func GetCurrentVersion(ctx context.Context, runner Runner, dir string, group ReleaseGroup) (*semver.Version, error) {
	inv, err := NewCurrentInvocation(group)
	if err != nil {
		return nil, err
	}

	stdout := new(bytes.Buffer)
	if err := runner.Run(ctx, dir, inv, stdout); err != nil {
		return nil, fmt.Errorf("execute current version command: %w", err)
	}

	currentVersionStr := strings.TrimSpace(stdout.String())
	currentSemver, err := semver.NewVersion(currentVersionStr)
	if err != nil {
		return nil, fmt.Errorf("%s: parse current version string: %w", currentVersionStr, err)
	}

	return currentSemver, nil
}

func GetNextVersion(ctx context.Context, runner Runner, dir string, group ReleaseGroup, level BumpLevel) (string, error) {
	currentSemver, err := GetCurrentVersion(ctx, runner, dir, group)
	if err != nil {
		return "", err
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
