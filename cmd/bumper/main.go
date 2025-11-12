package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"

	"github.com/disintegrator/bumper/buildinfo"
	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/builtins"
	"github.com/disintegrator/bumper/internal/commands/bump"
	"github.com/disintegrator/bumper/internal/commands/cat"
	"github.com/disintegrator/bumper/internal/commands/commit"
	"github.com/disintegrator/bumper/internal/commands/create"
	"github.com/disintegrator/bumper/internal/commands/current"
	"github.com/disintegrator/bumper/internal/commands/initialize"
	"github.com/disintegrator/bumper/internal/commands/next"
	"github.com/disintegrator/bumper/internal/o11y"
)

func newRootCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:    "bumper",
		Usage:   "A tool for managing versioning and changelogs",
		Version: buildinfo.Version,
		Authors: []any{"Georges Haidar (github.com/disintegrator)"},
		Commands: []*cli.Command{
			initialize.NewCommand(logger),
			create.NewCommand(logger),
			bump.NewCommand(logger),
			commit.NewCommand(logger),
			current.NewCommand(logger),
			next.NewCommand(logger),
			cat.NewCommand(logger),
			builtins.NewCommand(logger),
		},
	}
}

func main() {
	if err := mainErr(); err != nil {
		os.Exit(1)
	}
}

func mainErr() error {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := o11y.NewLogger()

	root := newRootCommand(logger)

	err := root.Run(ctx, os.Args)
	var cmdErr *cmd.CommandError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &cmdErr):
		// suppress logging, already logged
		return err
	default:
		fmt.Fprintln(os.Stderr, "")
		logger.ErrorContext(ctx, err.Error())
		return err
	}
}
