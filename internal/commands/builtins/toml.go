package builtins

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/semver/v3"
	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/urfave/cli/v3"
)

func newTOMLCurrentCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "current:toml",
		Usage: "Get the current version from a key in a TOML file",
		Flags: []cli.Flag{
			filePathFlag,
			keyFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			path := filePath(c)
			key := keyPath(c)

			segments, err := splitKeyPath(key)
			if err != nil {
				logger.ErrorContext(ctx, "invalid key path", slog.String("key", key), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				logger.ErrorContext(ctx, "failed to read toml file", slog.String("file", path), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			var doc map[string]any
			if err := toml.Unmarshal(data, &doc); err != nil {
				logger.ErrorContext(ctx, "failed to parse toml file", slog.String("file", path), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			var value any = doc
			for _, segment := range segments {
				table, ok := value.(map[string]any)
				if !ok {
					break
				}
				value, ok = table[segment]
				if !ok {
					value = nil
					break
				}
			}

			raw, ok := value.(string)
			if !ok {
				err := fmt.Errorf("key %q not found in %s or is not a string", key, path)
				logger.ErrorContext(ctx, "failed to find version key", slog.String("file", path), slog.String("key", key))
				return cmd.Failed(err)
			}

			sv, err := semver.NewVersion(raw)
			if err != nil {
				logger.ErrorContext(ctx, "failed to parse version", slog.String("version", raw), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fmt.Print(sv.String())

			return nil
		},
	}
}

// replaceTOMLValueInPlace swaps the version value on the line where it was
// parsed, keeping the rest of the file byte-for-byte intact. It reports false
// when the original value text cannot be located unambiguously on that line,
// in which case the caller must re-render the parsed document instead.
func replaceTOMLValueInPlace(data []byte, entry *tomledit.Entry, newValue string) ([]byte, bool) {
	oldValue := entry.KeyValue.Value.X.String()
	line := entry.KeyValue.Value.Line
	lines := strings.SplitAfter(string(data), "\n")
	if line < 1 || line > len(lines) || strings.Count(lines[line-1], oldValue) != 1 {
		return nil, false
	}
	lines[line-1] = strings.Replace(lines[line-1], oldValue, newValue, 1)
	return []byte(strings.Join(lines, "")), true
}

func newTOMLNextCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "next:toml",
		Usage: "Set the next version at a key in a TOML file",
		Flags: []cli.Flag{
			filePathsFlag,
			keyFlag,
			nextVersionFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			key := keyPath(c)
			version := nextVersion(c)

			segments, err := splitKeyPath(key)
			if err != nil {
				logger.ErrorContext(ctx, "invalid key path", slog.String("key", key), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			value, err := parser.ParseValue(strconv.Quote(version))
			if err != nil {
				logger.ErrorContext(ctx, "failed to build toml value", slog.String("version", version), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			for _, path := range filePaths(c) {
				data, err := os.ReadFile(path)
				if err != nil {
					logger.ErrorContext(ctx, "failed to read toml file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				doc, err := tomledit.Parse(bytes.NewReader(data))
				if err != nil {
					logger.ErrorContext(ctx, "failed to parse toml file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				entry := doc.First(segments...)
				if entry == nil || !entry.IsMapping() {
					err := fmt.Errorf("key %q not found in %s", key, path)
					logger.ErrorContext(ctx, "failed to find version key", slog.String("file", path), slog.String("key", key))
					return cmd.Failed(err)
				}

				updated, ok := replaceTOMLValueInPlace(data, entry, strconv.Quote(version))
				if !ok {
					// Fall back to re-rendering the whole document, which may
					// normalize formatting on unrelated lines.
					entry.KeyValue.Value.X = value.X

					var out bytes.Buffer
					var formatter tomledit.Formatter
					if err := formatter.Format(&out, doc); err != nil {
						logger.ErrorContext(ctx, "failed to render toml file", slog.String("file", path), slog.String("error", err.Error()))
						return cmd.Failed(err)
					}
					updated = out.Bytes()
				}

				err = os.WriteFile(path, updated, 0644)
				if err != nil {
					logger.ErrorContext(ctx, "failed to write toml file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				logger.InfoContext(ctx, "updated version", slog.String("file", path), slog.String("key", key), slog.String("version", version))
			}

			return nil
		},
	}
}
