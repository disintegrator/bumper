package builtins

import (
	"log/slog"

	"github.com/urfave/cli/v3"
)

func NewCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:    "builtins",
		Aliases: []string{"b"},
		Usage:   "Built-in commands for working with versions and changelogs",
		Commands: []*cli.Command{
			newDefaultCurrentCommand(logger),
			newFileCurrentCommand(logger),
			newNPMCurrentCommand(logger),

			newDefaultNextCommand(logger),
			newFileNextCommand(logger),
			newNPMNextCommand(logger),

			newDefaultAmendChangelogCommand(logger),
			newDefaultCatChangelogCommand(logger),
		},
	}
}
