package shared

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disintegrator/bumper/internal/cmd"
	"github.com/disintegrator/bumper/internal/workspace"
)

const validConfig = `
[[groups]]
name = "api"
display_name = "API"
changelog_cmd = ["true"]
cat_cmd = ["true"]
current_cmd = ["true"]
next_cmd = ["true"]

[[groups]]
name = "web"
display_name = "Web"
changelog_cmd = ["true"]
cat_cmd = ["true"]
current_cmd = ["true"]
next_cmd = ["true"]
`

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// setupWorkspace creates a temp workspace with the given config.toml content.
func setupWorkspace(t *testing.T, configTOML string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(workspace.Dir(dir), 0o755); err != nil {
		t.Fatalf("create .bumper dir: %v", err)
	}
	if err := os.WriteFile(workspace.ConfigFilename(dir), []byte(configTOML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return dir
}

// assertCommandError asserts err is a CommandError wrapping a real,
// descriptive cause — never a nil one, which would exit silently.
func assertCommandError(t *testing.T, err error, wantSubstr string) {
	t.Helper()

	if err == nil {
		t.Fatal("expected an error")
	}
	var cmdErr *cmd.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected a CommandError, got %T: %v", err, err)
	}
	cause := cmdErr.Unwrap()
	if cause == nil {
		t.Fatal("CommandError wraps a nil cause (silent exit)")
	}
	if !strings.Contains(cause.Error(), wantSubstr) {
		t.Errorf("cause = %q, want it to contain %q", cause.Error(), wantSubstr)
	}
}

func TestResolveMissingWorkspace(t *testing.T) {
	_, err := Resolve(t.Context(), discardLogger(), t.TempDir())
	assertCommandError(t, err, ".bumper directory not found")
}

func TestResolveMissingConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(workspace.Dir(dir), 0o755); err != nil {
		t.Fatalf("create .bumper dir: %v", err)
	}

	_, err := Resolve(t.Context(), discardLogger(), dir)
	assertCommandError(t, err, "config.toml")
}

func TestResolveInvalidConfig(t *testing.T) {
	dir := setupWorkspace(t, "[[groups]]\nname = \"api\"\n")

	_, err := Resolve(t.Context(), discardLogger(), dir)
	assertCommandError(t, err, "Invalid configuration")
}

func TestResolveValidWorkspace(t *testing.T) {
	dir := setupWorkspace(t, validConfig)

	res, err := Resolve(t.Context(), discardLogger(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Dir != dir {
		t.Errorf("dir = %q, want %q", res.Dir, dir)
	}
	if len(res.Config.Groups) != 2 {
		t.Errorf("groups = %d, want 2", len(res.Config.Groups))
	}
}

// Resolve finds the workspace by walking up from a nested directory.
func TestResolveFromNestedDir(t *testing.T) {
	dir := setupWorkspace(t, validConfig)
	nested := filepath.Join(dir, "packages", "api")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}

	res, err := Resolve(t.Context(), discardLogger(), nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Dir != dir {
		t.Errorf("dir = %q, want workspace root %q", res.Dir, dir)
	}
}

func TestResolutionGroup(t *testing.T) {
	singleGroup := &Resolution{Config: &workspace.Config{Groups: []workspace.ReleaseGroup{
		{Name: "api"},
	}}}
	multiGroup := &Resolution{Config: &workspace.Config{Groups: []workspace.ReleaseGroup{
		{Name: "api"},
		{Name: "web"},
	}}}
	noGroups := &Resolution{Config: &workspace.Config{}}

	tests := []struct {
		name       string
		res        *Resolution
		groupName  string
		wantGroup  string
		wantErrStr string
	}{
		{name: "default group selection", res: singleGroup, groupName: "", wantGroup: "api"},
		{name: "explicit group", res: multiGroup, groupName: "web", wantGroup: "web"},
		{name: "no groups defined", res: noGroups, groupName: "", wantErrStr: "no release groups defined"},
		{name: "no groups defined with name", res: noGroups, groupName: "api", wantErrStr: "no release groups defined"},
		{name: "multiple groups need a flag", res: multiGroup, groupName: "", wantErrStr: "--group flag is required"},
		{name: "unknown group", res: multiGroup, groupName: "mobile", wantErrStr: "release group not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, err := tt.res.Group(t.Context(), discardLogger(), tt.groupName)
			if tt.wantErrStr != "" {
				assertCommandError(t, err, tt.wantErrStr)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if group.Name != tt.wantGroup {
				t.Errorf("group = %q, want %q", group.Name, tt.wantGroup)
			}
		})
	}
}
