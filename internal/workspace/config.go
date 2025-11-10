package workspace

import "fmt"

type BumpLevel int

const (
	BumpLevelNone  BumpLevel = 0
	BumpLevelPatch BumpLevel = 1
	BumpLevelMinor BumpLevel = 2
	BumpLevelMajor BumpLevel = 3
)

func (b BumpLevel) String() string {
	switch b {
	case BumpLevelNone:
		return ""
	case BumpLevelPatch:
		return "patch"
	case BumpLevelMinor:
		return "minor"
	case BumpLevelMajor:
		return "major"
	default:
		panic(fmt.Sprintf("unknown bump level: %d", b))
	}
}

type Config struct {
	Groups []ReleaseGroup `json:"groups,omitempty,omitzero" toml:"groups,omitempty,omitzero" yaml:"groups,omitempty,omitzero"`
}

func (c *Config) IndexReleaseGroups() map[string]ReleaseGroup {
	result := make(map[string]ReleaseGroup, len(c.Groups))
	for _, g := range c.Groups {
		result[g.Name] = g
	}
	return result
}

type ReleaseGroup struct {
	Name         string   `json:"name" toml:"name" yaml:"name"`
	DisplayName  string   `json:"display_name,omitempty" toml:"display_name,omitempty" yaml:"display_name,omitempty"`
	ChangelogCMD []string `json:"changelog_cmd,omitempty,omitzero" toml:"changelog_cmd,omitempty,omitzero" yaml:"changelog_cmd,omitempty,omitzero"`
	CatCMD       []string `json:"cat_cmd,omitempty,omitzero" toml:"cat_cmd,omitempty,omitzero" yaml:"cat_cmd,omitempty,omitzero"`
	CurrentCMD   []string `json:"current_cmd,omitempty,omitzero" toml:"current_cmd,omitempty,omitzero" yaml:"current_cmd,omitempty,omitzero"`
	NextCMD      []string `json:"next_cmd,omitempty,omitzero" toml:"next_cmd,omitempty,omitzero" yaml:"next_cmd,omitempty,omitzero"`
}
