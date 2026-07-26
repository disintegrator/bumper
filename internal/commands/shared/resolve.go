package shared

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/workspace"
)

// Resolution is a resolved CLI invocation: the workspace directory containing
// .bumper and its validated config.
type Resolution struct {
	Dir    string
	Config *workspace.Config
}

// Resolve is the single entry point from a CLI invocation to a resolved
// workspace directory and validated config. rawDir is the --dir flag value;
// empty means start the workspace search from the current directory.
func Resolve(ctx context.Context, logger *slog.Logger, rawDir string) (*Resolution, error) {
	dir, err := workspace.GetWd(rawDir)
	if err != nil {
		logger.ErrorContext(ctx, "workspace directory not found", slog.String("dir", rawDir), slog.String("error", err.Error()))
		return nil, cmd.Failed(err)
	}

	cfg, err := LoadConfig(ctx, logger, dir)
	if err != nil {
		return nil, err
	}

	return &Resolution{Dir: dir, Config: cfg}, nil
}

// Group returns the release group selected by name, defaulting to the only
// group when exactly one is defined and name is empty.
func (r *Resolution) Group(ctx context.Context, logger *slog.Logger, name string) (workspace.ReleaseGroup, error) {
	if len(r.Config.Groups) == 0 {
		err := errors.New("no release groups defined in configuration")
		logger.ErrorContext(ctx, err.Error(), slog.String("hint", "use `bumper create` to create one"))
		return workspace.ReleaseGroup{}, cmd.Failed(err)
	}

	if name == "" {
		if len(r.Config.Groups) == 1 {
			return r.Config.Groups[0], nil
		}

		err := errors.New("--group flag is required when multiple release groups are defined")
		logger.ErrorContext(ctx, err.Error())
		return workspace.ReleaseGroup{}, cmd.Failed(err)
	}

	group, ok := r.Config.IndexReleaseGroups()[name]
	if !ok {
		err := fmt.Errorf("%s: release group not found in config", name)
		logger.ErrorContext(ctx, err.Error())
		return workspace.ReleaseGroup{}, cmd.Failed(err)
	}

	return group, nil
}
