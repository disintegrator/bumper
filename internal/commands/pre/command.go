package pre

import (
	"log/slog"

	"github.com/urfave/cli/v3"
)

func NewCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "pre",
		Usage: "Manage prerelease versions",
		Commands: []*cli.Command{
			newEnterCommand(logger),
			newExitCommand(logger),
			newStatusCommand(logger),
		},
	}
}
