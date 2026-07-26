package create

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/urfave/cli/v3"
)

func NewCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create a new release group",
		Arguments: []cli.Argument{
			&cli.StringArgs{
				Name: "groups",
				Min:  1,
				Max:  -1,
			},
		},
		Flags: []cli.Flag{
			shared.NewDirFlag(),
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			res, err := shared.Resolve(ctx, logger, shared.DirFlag(c))
			if err != nil {
				return err
			}
			cfg := res.Config

			names := c.StringArgs("groups")
			groups := make([]workspace.ReleaseGroup, 0, len(names))

			for _, name := range names {
				if slices.ContainsFunc(cfg.Groups, func(g workspace.ReleaseGroup) bool {
					return g.Name == name
				}) {
					logger.WarnContext(ctx, "skipping existing release group", slog.String("group", name))
					continue
				}

				groups = append(groups, workspace.ReleaseGroup{
					Name:         name,
					DisplayName:  name,
					ChangelogCMD: []string{"bumper", "builtins", "amendlog:default"},
					CatCMD:       []string{"bumper", "builtins", "cat:default"},
					CurrentCMD:   []string{"bumper", "builtins", "current:default"},
					NextCMD:      []string{"bumper", "builtins", "next:default"},
				})
			}

			if len(groups) == 0 {
				logger.InfoContext(ctx, "no new release groups to create")
				return nil
			}

			cfg.Groups = append(cfg.Groups, groups...)
			slices.SortStableFunc(cfg.Groups, func(a, b workspace.ReleaseGroup) int {
				return strings.Compare(a.Name, b.Name)
			})

			if err := shared.SaveConfig(ctx, logger, res.Dir, cfg); err != nil {
				return err
			}

			return nil
		},
	}
}
