package workspace

import (
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestExtractFrontMatter(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantLevels  map[string]string
		wantMessage string
		wantErr     bool
	}{
		{
			name:        "LF with trailing newline",
			content:     "---\napi: minor\n---\n\nAdded a feature\n",
			wantLevels:  map[string]string{"api": "minor"},
			wantMessage: "Added a feature",
		},
		{
			name:        "CRLF line endings",
			content:     "---\r\napi: minor\r\n---\r\n\r\nAdded a feature\r\n",
			wantLevels:  map[string]string{"api": "minor"},
			wantMessage: "Added a feature",
		},
		{
			name:        "missing trailing newline",
			content:     "---\napi: minor\n---\n\nAdded a feature",
			wantLevels:  map[string]string{"api": "minor"},
			wantMessage: "Added a feature",
		},
		{
			name:        "closing delimiter without trailing newline",
			content:     "---\napi: minor\n---",
			wantLevels:  map[string]string{"api": "minor"},
			wantMessage: "",
		},
		{
			name:        "CRLF without trailing newline",
			content:     "---\r\napi: major\r\n---\r\n\r\nBreaking",
			wantLevels:  map[string]string{"api": "major"},
			wantMessage: "Breaking",
		},
		{
			name:        "empty front matter",
			content:     "---\n---\n",
			wantLevels:  map[string]string{},
			wantMessage: "",
		},
		{
			name:        "multiline message keeps interior blank lines",
			content:     "---\napi: patch\n---\n\nFirst paragraph\n\nSecond paragraph\n",
			wantLevels:  map[string]string{"api": "patch"},
			wantMessage: "First paragraph\n\nSecond paragraph",
		},
		{
			name:    "missing opening delimiter",
			content: "api: minor\n---\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			levels := make(map[string]string)
			message, err := extractFrontMatter(tt.content, &levels)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(levels, tt.wantLevels) {
				t.Errorf("levels = %#v, want %#v", levels, tt.wantLevels)
			}
			if message != tt.wantMessage {
				t.Errorf("message = %q, want %q", message, tt.wantMessage)
			}
		})
	}
}

func commitAt(sha string, unixSec int64) *Commit {
	return &Commit{SHA: sha, When: time.Unix(unixSec, 0)}
}

func TestSquashBumps(t *testing.T) {
	cfg := &Config{Groups: []ReleaseGroup{{Name: "api"}, {Name: "web"}}}
	logger := slog.New(slog.DiscardHandler)

	t.Run("highest level wins", func(t *testing.T) {
		bumps := []ParsedBump{
			{File: "bump-1.md", Levels: map[string]string{"api": "minor"}, Message: "feature"},
			{File: "bump-2.md", Levels: map[string]string{"api": "major"}, Message: "breaking"},
			{File: "bump-3.md", Levels: map[string]string{"api": "patch"}, Message: "fix"},
		}

		statuses := SquashBumps(t.Context(), logger, bumps, cfg)

		status := statuses["api"]
		if status == nil {
			t.Fatal("expected a status for api")
		}
		if status.Level != BumpLevelMajor {
			t.Errorf("level = %v, want %v", status.Level, BumpLevelMajor)
		}
		if len(status.MajorLogs) != 1 || len(status.MinorLogs) != 1 || len(status.PatchLogs) != 1 {
			t.Errorf("logs = %d/%d/%d, want 1 entry per level", len(status.MajorLogs), len(status.MinorLogs), len(status.PatchLogs))
		}
	})

	t.Run("one bump file spanning multiple groups", func(t *testing.T) {
		bumps := []ParsedBump{
			{File: "bump-1.md", Levels: map[string]string{"api": "minor", "web": "patch"}, Message: "shared change"},
		}

		statuses := SquashBumps(t.Context(), logger, bumps, cfg)

		if statuses["api"] == nil || statuses["api"].Level != BumpLevelMinor {
			t.Errorf("api status = %+v, want minor", statuses["api"])
		}
		if statuses["web"] == nil || statuses["web"].Level != BumpLevelPatch {
			t.Errorf("web status = %+v, want patch", statuses["web"])
		}
		if len(statuses["api"].MinorLogs) != 1 || statuses["api"].MinorLogs[0].Content == "" {
			t.Error("expected the shared entry in api's minor logs")
		}
	})

	t.Run("entries ordered by commit timestamp with sha prefixes", func(t *testing.T) {
		bumps := []ParsedBump{
			{File: "bump-1.md", Levels: map[string]string{"api": "patch"}, Message: "third", Commit: commitAt("ccccccc1111", 300)},
			{File: "bump-2.md", Levels: map[string]string{"api": "patch"}, Message: "first", Commit: commitAt("aaaaaaa2222", 100)},
			{File: "bump-3.md", Levels: map[string]string{"api": "patch"}, Message: "second", Commit: commitAt("bbbbbbb3333", 200)},
		}

		statuses := SquashBumps(t.Context(), logger, bumps, cfg)

		var contents []string
		for _, entry := range statuses["api"].PatchLogs {
			contents = append(contents, entry.Content)
		}
		want := []string{"aaaaaaa: first", "bbbbbbb: second", "ccccccc: third"}
		if !reflect.DeepEqual(contents, want) {
			t.Errorf("patch logs = %v, want %v", contents, want)
		}
	})

	t.Run("unresolved provenance sorts first without prefix", func(t *testing.T) {
		bumps := []ParsedBump{
			{File: "bump-1.md", Levels: map[string]string{"api": "patch"}, Message: "committed", Commit: commitAt("aaaaaaa2222", 100)},
			{File: "bump-2.md", Levels: map[string]string{"api": "patch"}, Message: "uncommitted"},
		}

		statuses := SquashBumps(t.Context(), logger, bumps, cfg)

		logs := statuses["api"].PatchLogs
		if len(logs) != 2 {
			t.Fatalf("patch logs = %d, want 2", len(logs))
		}
		if logs[0].Content != "uncommitted" || logs[0].Commit != "" {
			t.Errorf("first entry = %+v, want unprefixed uncommitted entry", logs[0])
		}
	})

	t.Run("unknown groups and levels are skipped", func(t *testing.T) {
		bumps := []ParsedBump{
			{File: "bump-1.md", Levels: map[string]string{"mobile": "minor"}, Message: "for unknown group"},
			{File: "bump-2.md", Levels: map[string]string{"api": "gigantic"}, Message: "unknown level"},
		}

		statuses := SquashBumps(t.Context(), logger, bumps, cfg)

		if _, ok := statuses["mobile"]; ok {
			t.Error("unknown group must not produce a status")
		}
		if status := statuses["api"]; status != nil && status.Level != BumpLevelNone {
			t.Errorf("api level = %v, want none for unknown level", status.Level)
		}
	})
}

// CollectBumps composes gather and squash: files on disk plus fake provenance
// in, ordered per-group statuses out.
func TestCollectBumpsComposesGatherAndSquash(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(Dir(dir), 0o755); err != nil {
		t.Fatalf("create .bumper dir: %v", err)
	}

	newer := BumpFilename(dir, "newer")
	older := BumpFilename(dir, "older")
	if err := os.WriteFile(newer, []byte("---\napi: major\n---\n\nBreaking change"), 0o644); err != nil {
		t.Fatalf("write bump: %v", err)
	}
	// CRLF file, exercising the front-matter fix through the composed path.
	if err := os.WriteFile(older, []byte("---\r\napi: minor\r\n---\r\n\r\nOld feature\r\n"), 0o644); err != nil {
		t.Fatalf("write bump: %v", err)
	}

	cfg := &Config{Groups: []ReleaseGroup{{Name: "api"}}}
	prov := &FakeProvenance{Commits: map[string]Commit{
		newer: {SHA: "fffffff9999", When: time.Unix(900, 0)},
		older: {SHA: "1111111aaaa", When: time.Unix(100, 0)},
	}}
	logger := slog.New(slog.DiscardHandler)

	statuses, err := CollectBumps(t.Context(), logger, dir, cfg, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := statuses["api"]
	if status == nil {
		t.Fatal("expected a status for api")
	}
	if status.Level != BumpLevelMajor {
		t.Errorf("level = %v, want major", status.Level)
	}
	if len(status.MinorLogs) != 1 || status.MinorLogs[0].Content != "1111111: Old feature" {
		t.Errorf("minor logs = %+v, want the CRLF entry with sha prefix", status.MinorLogs)
	}
	if len(status.MajorLogs) != 1 || status.MajorLogs[0].Content != "fffffff: Breaking change" {
		t.Errorf("major logs = %+v, want the newline-less entry with sha prefix", status.MajorLogs)
	}
}
