package workspace

import (
	"fmt"
	"strings"
)

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

type invalidReleaseGroupConfigError struct {
	name  string
	index int
	errs  []error
}

func (e *invalidReleaseGroupConfigError) Error() string {
	lines := make([]string, 0, len(e.errs)+1)
	lines = append(lines, fmt.Sprintf("  - Invalid configuration for release group %q (index %d):", e.name, e.index))
	for _, err := range e.errs {
		lines = append(lines, fmt.Sprintf("    * %s", err.Error()))
	}

	return strings.Join(lines, "\n")
}

type InvalidConfigError struct {
	globalErrors []error
	groupErrors  []*invalidReleaseGroupConfigError
}

func (e *InvalidConfigError) Error() string {
	var builder strings.Builder
	builder.WriteString("Invalid workspace configuration:\n")

	for _, ge := range e.globalErrors {
		builder.WriteString("  - " + ge.Error() + "\n")
	}

	for _, ge := range e.groupErrors {
		builder.WriteString(ge.Error() + "\n")
	}

	return strings.TrimSpace(builder.String())
}

func validateConfig(cfg *Config) error {
	var globalErrors []error
	groupErrors := []*invalidReleaseGroupConfigError{}

	seenNames := make(map[string]struct{})

	for i, group := range cfg.Groups {
		gerr := invalidReleaseGroupConfigError{
			name:  group.Name,
			index: i,
			errs:  []error{},
		}

		if group.Name == "" {
			gerr.errs = append(gerr.errs, fmt.Errorf("no name is set"))
		}

		if _, exists := seenNames[group.Name]; group.Name != "" && exists {
			msg := "Duplicate release group name found"
			globalErrors = append(globalErrors, fmt.Errorf("%s: %q", msg, group.Name))
		}
		seenNames[group.Name] = struct{}{}

		if len(group.ChangelogCMD) == 0 {
			gerr.errs = append(gerr.errs, fmt.Errorf("no changelog command defined"))
		}
		if len(group.CatCMD) == 0 {
			gerr.errs = append(gerr.errs, fmt.Errorf("no cat command defined"))
		}
		if len(group.CurrentCMD) == 0 {
			gerr.errs = append(gerr.errs, fmt.Errorf("no current command defined"))
		}
		if len(group.NextCMD) == 0 {
			gerr.errs = append(gerr.errs, fmt.Errorf("no next command defined"))
		}

		if len(gerr.errs) > 0 {
			groupErrors = append(groupErrors, &gerr)
		}
	}

	if len(globalErrors) > 0 || len(groupErrors) > 0 {
		return &InvalidConfigError{
			globalErrors: globalErrors,
			groupErrors:  groupErrors,
		}
	}

	return nil
}
