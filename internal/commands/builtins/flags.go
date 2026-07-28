package builtins

import (
	"fmt"
	"slices"
	"strings"

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

var keyFlag = &cli.StringFlag{
	Name:     "key",
	Usage:    "Dot-separated path to the version field within the file (e.g. 'package.version')",
	Required: true,
}

func keyPath(c *cli.Command) string {
	return c.String("key")
}

// splitKeyPath breaks a dot-separated key path into its segments. Escaping is
// not supported: keys that themselves contain dots cannot be addressed.
func splitKeyPath(key string) ([]string, error) {
	segments := strings.Split(key, ".")
	if slices.Contains(segments, "") {
		return nil, fmt.Errorf("invalid key path %q: empty segment", key)
	}
	return segments, nil
}

func releaseGroup(c *cli.Command) string {
	return c.String("group")
}

func version(c *cli.Command) string {
	return c.String("version")
}

func nextVersion(c *cli.Command) string {
	return c.String("version")
}
