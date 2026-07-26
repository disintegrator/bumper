// Package changelog implements the markdown surgery behind the default
// changelog strategy: inserting a release section under the top-level
// "# Changelog" heading and extracting the section for a given release.
// All operations read from and write to streams; callers own file handling.
package changelog

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Release describes a release section to insert into a changelog.
type Release struct {
	DisplayName string
	Version     string
	Major       []string
	Minor       []string
	Patch       []string
}

// heading is the top-level heading a changelog must start with; release
// sections are inserted immediately below it.
const heading = "# Changelog"

// whitespaceOnlyLine matches lines containing only whitespace, produced when
// entry continuation indentation is applied to blank lines.
var whitespaceOnlyLine = regexp.MustCompile(`(?m)^\s+$`)

// Amend copies the changelog from r to w, inserting the release's section
// directly under the "# Changelog" heading. Empty input is initialized with
// the heading first. Input that has content but no heading is copied through
// unchanged, without the new section.
func Amend(w io.Writer, r io.Reader, release Release) error {
	section := release.render()

	out := bufio.NewWriter(w)
	scanner := bufio.NewScanner(r)
	sawInput := false
	inserted := false

	for scanner.Scan() {
		sawInput = true
		line := scanner.Text()

		if _, err := out.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("write changelog: %w", err)
		}

		if !inserted && strings.HasPrefix(line, heading) {
			if _, err := out.WriteString("\n" + section + "\n"); err != nil {
				return fmt.Errorf("write changelog: %w", err)
			}
			inserted = true
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read changelog: %w", err)
	}

	if !sawInput {
		if _, err := out.WriteString(heading + "\n\n" + section + "\n"); err != nil {
			return fmt.Errorf("write changelog: %w", err)
		}
	}

	if err := out.Flush(); err != nil {
		return fmt.Errorf("flush changelog: %w", err)
	}

	return nil
}

// Section reads the changelog from r and returns the section belonging to the
// release titled "## <displayName> <version>", without surrounding blank
// lines. It returns an empty string when the release is not present.
func Section(r io.Reader, displayName string, version string) (string, error) {
	title := fmt.Sprintf("## %s %s", displayName, version)

	scanner := bufio.NewScanner(r)
	collecting := false
	var output strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if collecting && strings.HasPrefix(line, "## ") {
			break
		}
		if !collecting && !strings.HasPrefix(line, title) {
			continue
		}

		collecting = true
		output.WriteString(line)
		output.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read changelog: %w", err)
	}

	return strings.TrimSpace(output.String()), nil
}

func (r Release) render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s %s\n\n", r.DisplayName, r.Version)

	levels := []struct {
		title   string
		entries []string
	}{
		{"Major Changes", r.Major},
		{"Minor Changes", r.Minor},
		{"Patch Changes", r.Patch},
	}
	for _, level := range levels {
		if len(level.entries) == 0 {
			continue
		}

		b.WriteString("### ")
		b.WriteString(level.title)
		b.WriteString("\n\n")
		for _, entry := range level.entries {
			b.WriteString(formatEntry(entry))
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

// formatEntry renders one changelog entry as a markdown list item, indenting
// continuation lines and stripping indentation from blank ones.
func formatEntry(entry string) string {
	if entry == "" {
		return ""
	}

	entry = "- " + strings.ReplaceAll(entry, "\n", "\n  ")
	entry = whitespaceOnlyLine.ReplaceAllString(entry, "")

	return entry + "\n"
}
