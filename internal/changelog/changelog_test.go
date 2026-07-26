package changelog

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files with actual output")

func readFixture(t *testing.T, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	return string(content)
}

func TestAmendGolden(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		golden  string
		release Release
	}{
		{
			name:   "empty changelog with entries at every level",
			input:  "amend-empty.input.md",
			golden: "amend-empty.golden.md",
			release: Release{
				DisplayName: "API",
				Version:     "1.0.0",
				Major:       []string{"Breaking change"},
				Minor:       []string{"New feature"},
				Patch:       []string{"Bug fix"},
			},
		},
		{
			name:   "existing releases get the new section on top",
			input:  "amend-existing.input.md",
			golden: "amend-existing.golden.md",
			release: Release{
				DisplayName: "API",
				Version:     "1.0.1",
				Patch:       []string{"7ecb5a4: Fixed a bug"},
			},
		},
		{
			name:   "multiline and blank-line entries",
			input:  "amend-empty.input.md",
			golden: "amend-multiline.golden.md",
			release: Release{
				DisplayName: "API",
				Version:     "2.0.0",
				Major:       []string{"Dropped legacy support.\nMigration guide:\nhttps://example.com/migrate"},
				Patch:       []string{"Fixed a race\n\nwith a blank line in the entry"},
			},
		},
		{
			name:   "input without a changelog heading is copied through unchanged",
			input:  "amend-no-heading.input.md",
			golden: "amend-no-heading.input.md",
			release: Release{
				DisplayName: "API",
				Version:     "1.0.0",
				Patch:       []string{"Bug fix"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := readFixture(t, tt.input)

			var got bytes.Buffer
			if err := Amend(&got, strings.NewReader(input), tt.release); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if *update && tt.golden != tt.input {
				if err := os.WriteFile(filepath.Join("testdata", tt.golden), got.Bytes(), 0o644); err != nil {
					t.Fatalf("update golden %s: %v", tt.golden, err)
				}
			}

			want := readFixture(t, tt.golden)
			if got.String() != want {
				t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got.String(), want)
			}
		})
	}
}

func TestSection(t *testing.T) {
	input := readFixture(t, "section.input.md")

	tests := []struct {
		name        string
		displayName string
		version     string
		want        string
	}{
		{
			name:        "first release",
			displayName: "API",
			version:     "1.1.0",
			want:        "## API 1.1.0\n\n### Minor Changes\n\n- Second feature",
		},
		{
			name:        "middle release",
			displayName: "API",
			version:     "1.0.1",
			want:        "## API 1.0.1\n\n### Patch Changes\n\n- 7ecb5a4: Fixed a bug",
		},
		{
			name:        "last release runs to end of file",
			displayName: "API",
			version:     "1.0.0",
			want:        "## API 1.0.0\n\n### Minor Changes\n\n- New feature",
		},
		{
			name:        "missing version",
			displayName: "API",
			version:     "9.9.9",
			want:        "",
		},
		{
			name:        "wrong display name",
			displayName: "Web",
			version:     "1.0.0",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Section(strings.NewReader(input), tt.displayName, tt.version)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("section = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatEntry(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		want  string
	}{
		{name: "empty entry produces nothing", entry: "", want: ""},
		{name: "single line", entry: "Fixed a bug", want: "- Fixed a bug\n"},
		{
			name:  "continuation lines are indented",
			entry: "First line\nsecond line",
			want:  "- First line\n  second line\n",
		},
		{
			name:  "blank lines are stripped of indentation",
			entry: "First paragraph\n\nsecond paragraph",
			want:  "- First paragraph\n\n  second paragraph\n",
		},
		{
			name:  "whitespace-only lines are emptied",
			entry: "First\n \t\nsecond",
			want:  "- First\n\n  second\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatEntry(tt.entry); got != tt.want {
				t.Errorf("formatEntry(%q) = %q, want %q", tt.entry, got, tt.want)
			}
		})
	}
}
