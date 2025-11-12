package bump

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/commands/shared"
	"github.com/disintegrator/bumper/internal/random"
	"github.com/disintegrator/bumper/internal/workspace"
	"github.com/goccy/go-yaml"
	"github.com/urfave/cli/v3"
)

type bumpOptions struct {
	groups  []string
	level   string
	message string
}

func NewCommand(logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "bump",
		Usage: "Bump the version for one or more release groups",
		Flags: []cli.Flag{
			shared.NewDirFlag(),
			&cli.BoolFlag{
				Name: "empty",
				Usage: "Create an empty bump file without prompting for any input." +
					" Useful for passing CI/CD checks that require bump files.",
			},
			&cli.StringSliceFlag{
				Name:  "group",
				Usage: "The release groups to bump",
			},
			&cli.BoolFlag{
				Name:  "major",
				Usage: "Bump the major version. This has highest precedence over other level flags.",
			},
			&cli.BoolFlag{
				Name:  "minor",
				Usage: "Bump the minor version.",
			},
			&cli.BoolFlag{
				Name:  "patch",
				Usage: "Bump the patch version.",
			},
			&cli.StringFlag{
				Name:    "message",
				Aliases: []string{"m"},
				Usage:   "The changelog entry to use for the bump.",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			rawdir := shared.DirFlag(c)
			dir, err := workspace.GetWd(rawdir)
			if err != nil {
				logger.ErrorContext(ctx, "workspace directory not found", slog.String("dir", rawdir), slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			cfg, err := shared.LoadConfig(ctx, logger, dir)
			if err != nil {
				return err
			}

			if c.Bool("empty") {
				filename := workspace.BumpFilename(dir, random.GetRandomName())
				logger.InfoContext(ctx, "Creating empty bump file", slog.String("file", filename))
				f, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
				if err != nil {
					logger.ErrorContext(ctx, "failed to create bump file", slog.String("error", err.Error()))
					return cmd.Failed(err)
				}
				defer f.Close()

				_, err = f.WriteString("---\n---\n")
				if err != nil {
					logger.ErrorContext(ctx, "failed to write bump file", slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				return nil
			}

			groupsFlag := c.StringSlice("group")
			messageFlag := c.String("message")
			levelFlag := ""
			switch {
			case c.Bool("major"):
				levelFlag = "major"
			case c.Bool("minor"):
				levelFlag = "minor"
			case c.Bool("patch"):
				levelFlag = "patch"
			}

			indexed := cfg.IndexReleaseGroups()
			if len(groupsFlag) > 0 {
				for _, g := range groupsFlag {
					if _, ok := indexed[g]; !ok {
						logger.ErrorContext(ctx, "release group not found in config", slog.String("group", g))
						return cmd.Failed(fmt.Errorf("%s: release group not found in config", g))
					}
				}
			}

			deduped := make(map[string]struct{})
			for _, g := range groupsFlag {
				deduped[g] = struct{}{}
			}
			groupsDeduped := slices.Sorted(maps.Keys(deduped))

			bumpOpts := &bumpOptions{
				groups:  groupsDeduped,
				level:   levelFlag,
				message: messageFlag,
			}

			groupOpts := make([]huh.Option[string], 0, len(cfg.Groups))
			for _, g := range cfg.Groups {
				groupOpts = append(groupOpts, huh.NewOption(g.Name, g.Name))
			}
			if len(groupOpts) == 0 {
				logger.ErrorContext(ctx, "no release groups defined. use `bumper create` to create some.")
				return cmd.Failed(errors.New("no release groups defined"))
			}

			if len(cfg.Groups) == 1 && len(bumpOpts.groups) == 0 {
				bumpOpts.groups = []string{cfg.Groups[0].Name}
			}

			err = huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Options(groupOpts...).
						Filterable(true).
						Validate(func(s []string) error {
							if len(s) == 0 {
								return errors.New("at least one release group must be selected")
							}
							return nil
						}).
						Value(&bumpOpts.groups),
				).WithHideFunc(func() bool {
					return len(groupsDeduped) > 0 || len(groupOpts) < 2
				}),

				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Choose a level").
						Options(
							huh.NewOption("major", "major"),
							huh.NewOption("minor", "minor"),
							huh.NewOption("patch", "patch"),
						).
						Value(&bumpOpts.level),
				).WithHideFunc(func() bool {
					return len(bumpOpts.groups) == 0 || levelFlag != ""
				}),

				huh.NewGroup(
					huh.NewText().
						Title("Changelog entry message").
						Placeholder("Describe the changes in this version. Markdown is supported.").
						ExternalEditor(true).
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return errors.New("changelog message cannot be empty")
							}

							return nil
						}).
						Value(&bumpOpts.message),
				).WithHideFunc(func() bool {
					return len(bumpOpts.groups) == 0 || strings.TrimSpace(messageFlag) != ""
				}),
			).RunWithContext(ctx)
			if err != nil {
				logger.ErrorContext(ctx, "failed to get changelog message", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			bumps := make(map[string]string)
			for _, groupName := range bumpOpts.groups {
				bumps[groupName] = bumpOpts.level
			}
			ymlbs, err := yaml.Marshal(bumps)
			if err != nil {
				logger.ErrorContext(ctx, "failed to marshal bumps to frontmatter", slog.String("error", err.Error()))
				return cmd.Failed(err)
			}

			fm := string(ymlbs)

			content := fmt.Sprintf("---\n%s---\n\n%s\n", fm, bumpOpts.message)

			var existsErr error
			for range 3 {
				existsErr = nil
				filename := workspace.BumpFilename(dir, random.GetRandomName())
				fmt.Println("Creating bump file:", filename)
				f, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
				switch {
				case errors.Is(err, os.ErrExist):
					existsErr = err
					continue
				case err != nil:
					logger.ErrorContext(ctx, "failed to create bump file", slog.String("error", err.Error()))
					return cmd.Failed(err)
				}
				defer f.Close()

				_, err = f.WriteString(content)
				if err != nil {
					logger.ErrorContext(ctx, "failed to write bump file", slog.String("error", err.Error()))
					return cmd.Failed(err)
				}

				logger.InfoContext(ctx, "Bump file created successfully", slog.String("file", filename))

				break
			}
			if existsErr != nil {
				logger.ErrorContext(ctx, "failed to create bump file after several attempts", slog.String("error", existsErr.Error()))
				return cmd.Failed(err)
			}

			return nil
		},
	}
}
