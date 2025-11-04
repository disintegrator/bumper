package builtins

import (
	"github.com/Masterminds/semver/v3"
	"github.com/urfave/cli/v3"
)

var (
	releaseGroupFlag = &cli.StringFlag{
		Name:     "group",
		Usage:    "Release group name",
		Required: true,
		Sources:  cli.EnvVars("BUMPER_GROUP"),
	}

	versionFlag = &cli.StringFlag{
		Name:     "version",
		Usage:    "A version to query for",
		Required: true,
		Sources:  cli.EnvVars("BUMPER_GROUP_VERSION"),
		Validator: func(s string) error {
			_, err := semver.NewVersion(s)
			return err
		},
	}

	nextVersionFlag = &cli.StringFlag{
		Name:     "version",
		Usage:    "The new version to set",
		Required: true,
		Sources:  cli.EnvVars("BUMPER_GROUP_NEXT_VERSION"),
		Validator: func(s string) error {
			_, err := semver.NewVersion(s)
			return err
		},
	}
)

func releaseGroup(c *cli.Command) string {
	return c.String("group")
}

func version(c *cli.Command) string {
	return c.String("version")
}

func nextVersion(c *cli.Command) string {
	return c.String("version")
}
