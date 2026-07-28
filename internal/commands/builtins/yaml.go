package builtins

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/goccy/go-yaml"
	yamlparser "github.com/goccy/go-yaml/parser"
	"github.com/urfave/cli/v3"
)

// yamlVersionPath converts a dot-separated key path into a goccy/go-yaml path
// (e.g. "package.version" -> "$.package.version").
func yamlVersionPath(key string) (*yaml.Path, error) {
	segments, err := splitKeyPath(key)
	if err != nil {
		return nil, err
	}

	builder := (&yaml.PathBuilder{}).Root()
	for _, segment := range segments {
		builder = builder.Child(segment)
	}

	return builder.Build(), nil
}

func newYAMLCurrentCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "current:yaml",
		Usage: "Get the current version from a key in a YAML file",
		Flags: []cli.Flag{
			filePathFlag,
			keyFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			path := filePath(c)
			key := keyPath(c)

			versionPath, err := yamlVersionPath(key)
			if err != nil {
				logger.ErrorContext(ctx, "invalid key path", slog.String("key", key), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				logger.ErrorContext(ctx, "failed to read yaml file", slog.String("file", path), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			var raw string
			if err := versionPath.Read(bytes.NewReader(data), &raw); err != nil {
				logger.ErrorContext(ctx, "failed to read version key", slog.String("file", path), slog.String("key", key), slog.String("error", err.Error()))
				return cmd.Failed(fmt.Errorf("read key %q in %s: %w", key, path, err))
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

func newYAMLNextCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "next:yaml",
		Usage: "Set the next version at a key in a YAML file",
		Flags: []cli.Flag{
			filePathsFlag,
			keyFlag,
			nextVersionFlag,
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			key := keyPath(c)
			version := nextVersion(c)

			versionPath, err := yamlVersionPath(key)
			if err != nil {
				logger.ErrorContext(ctx, "invalid key path", slog.String("key", key), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			for _, path := range filePaths(c) {
				data, err := os.ReadFile(path)
				if err != nil {
					logger.ErrorContext(ctx, "failed to read yaml file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				file, err := yamlparser.ParseBytes(data, yamlparser.ParseComments)
				if err != nil {
					logger.ErrorContext(ctx, "failed to parse yaml file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				node, err := versionPath.FilterFile(file)
				if err != nil {
					logger.ErrorContext(ctx, "failed to find version key", slog.String("file", path), slog.String("key", key), slog.String("error", err.Error()))
					return cmd.Failed(fmt.Errorf("find key %q in %s: %w", key, path, err))
				}
				comment := node.GetComment()

				if err := versionPath.ReplaceWithReader(file, strings.NewReader(version)); err != nil {
					logger.ErrorContext(ctx, "failed to set version in yaml file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				// Replacement discards any line comment that trailed the old
				// value; carry it over to the new node.
				if comment != nil {
					if node, err := versionPath.FilterFile(file); err == nil {
						_ = node.SetComment(comment)
					}
				}

				out := file.String()
				if !strings.HasSuffix(out, "\n") {
					out += "\n"
				}

				err = os.WriteFile(path, []byte(out), 0644)
				if err != nil {
					logger.ErrorContext(ctx, "failed to write yaml file", slog.String("file", path), slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				logger.InfoContext(ctx, "updated version", slog.String("file", path), slog.String("key", key), slog.String("version", version))
			}

			return nil
		},
	}
}
