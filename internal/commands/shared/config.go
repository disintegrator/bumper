package shared

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/workspace"
)

func LoadConfig(ctx context.Context, logger *slog.Logger, workspaceDir string) (*workspace.Config, error) {
	cfg, err := workspace.LoadConfig(workspaceDir)
	var invalidErr *workspace.InvalidConfigError
	switch {
	case errors.As(err, &invalidErr):
		fmt.Fprintln(os.Stderr, invalidErr.Error())
		return nil, cmd.Failed(err)
	case err != nil:
		logger.ErrorContext(ctx, "failed to load config", slog.String("dir", workspaceDir), slog.String("error", err.Error()))
		return nil, cmd.Failed(err)
	}

	return cfg, nil
}

func SaveConfig(ctx context.Context, logger *slog.Logger, workspaceDir string, cfg *workspace.Config) error {
	err := workspace.SaveConfig(workspaceDir, cfg)
	var invalidErr *workspace.InvalidConfigError
	switch {
	case errors.As(err, &invalidErr):
		fmt.Fprintln(os.Stderr, invalidErr.Error())
		return cmd.Failed(err)
	case err != nil:
		logger.ErrorContext(ctx, "failed to save config", slog.String("dir", workspaceDir), slog.String("error", err.Error()))
		return cmd.Failed(err)
	}

	return nil
}
