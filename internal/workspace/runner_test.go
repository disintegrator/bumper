package workspace

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestGroupInvocations(t *testing.T) {
	group := ReleaseGroup{
		Name:         "api",
		CurrentCMD:   []string{"bumper", "builtins", "current:file", "--file", "VERSION"},
		NextCMD:      []string{"bumper", "builtins", "next:file", "--file", "VERSION"},
		ChangelogCMD: []string{"bumper", "builtins", "changelog:default"},
		CatCMD:       []string{"bumper", "builtins", "cat:default"},
	}

	status := &ReleaseGroupStatus{
		Level:     BumpLevelMajor,
		MajorLogs: []LogEntry{{Content: "abc1234: breaking change"}},
		MinorLogs: []LogEntry{{Content: "def5678: new feature"}, {Content: "aaa0000: another feature"}},
		PatchLogs: []LogEntry{{Content: "bbb1111: bug fix"}},
	}

	tests := []struct {
		name     string
		invoke   func() (GroupInvocation, error)
		wantVerb Verb
		wantArgv []string
		wantEnv  []string
	}{
		{
			name:     "current",
			invoke:   func() (GroupInvocation, error) { return NewCurrentInvocation(group) },
			wantVerb: VerbCurrent,
			wantArgv: []string{"bumper", "builtins", "current:file", "--file", "VERSION"},
			wantEnv:  []string{"BUMPER_GROUP=api"},
		},
		{
			name:     "next",
			invoke:   func() (GroupInvocation, error) { return NewNextInvocation(group, "2.0.0") },
			wantVerb: VerbNext,
			wantArgv: []string{"bumper", "builtins", "next:file", "--file", "VERSION"},
			wantEnv:  []string{"BUMPER_GROUP=api", "BUMPER_GROUP_NEXT_VERSION=2.0.0"},
		},
		{
			name:     "changelog",
			invoke:   func() (GroupInvocation, error) { return NewChangelogInvocation(group, "2.0.0", status) },
			wantVerb: VerbChangelog,
			wantArgv: []string{
				"bumper", "builtins", "changelog:default",
				"--group", "api",
				"--major", "abc1234: breaking change",
				"--minor", "def5678: new feature",
				"--minor", "aaa0000: another feature",
				"--patch", "bbb1111: bug fix",
			},
			wantEnv: []string{"BUMPER_GROUP=api", "BUMPER_GROUP_NEXT_VERSION=2.0.0"},
		},
		{
			name:     "cat",
			invoke:   func() (GroupInvocation, error) { return NewCatInvocation(group, "1.2.0") },
			wantVerb: VerbCat,
			wantArgv: []string{"bumper", "builtins", "cat:default"},
			wantEnv:  []string{"BUMPER_GROUP=api", "BUMPER_GROUP_VERSION=1.2.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv, err := tt.invoke()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if inv.Verb != tt.wantVerb {
				t.Errorf("verb = %q, want %q", inv.Verb, tt.wantVerb)
			}
			if !reflect.DeepEqual(inv.Argv, tt.wantArgv) {
				t.Errorf("argv = %#v, want %#v", inv.Argv, tt.wantArgv)
			}
			if !reflect.DeepEqual(inv.Env, tt.wantEnv) {
				t.Errorf("env = %#v, want %#v", inv.Env, tt.wantEnv)
			}
		})
	}
}

func TestGroupInvocationsMissingCommand(t *testing.T) {
	group := ReleaseGroup{Name: "api"}
	status := &ReleaseGroupStatus{}

	tests := []struct {
		name   string
		invoke func() (GroupInvocation, error)
	}{
		{"current", func() (GroupInvocation, error) { return NewCurrentInvocation(group) }},
		{"next", func() (GroupInvocation, error) { return NewNextInvocation(group, "1.0.0") }},
		{"changelog", func() (GroupInvocation, error) { return NewChangelogInvocation(group, "1.0.0", status) }},
		{"cat", func() (GroupInvocation, error) { return NewCatInvocation(group, "1.0.0") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.invoke(); err == nil {
				t.Error("expected error for group with no command configured")
			}
		})
	}
}

func TestGetCurrentVersion(t *testing.T) {
	group := ReleaseGroup{
		Name:       "api",
		CurrentCMD: []string{"print-version"},
	}
	runner := &FakeRunner{Stdout: map[Verb]string{VerbCurrent: "1.2.3\n"}}

	version, err := GetCurrentVersion(t.Context(), runner, "/repo", group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := version.String(); got != "1.2.3" {
		t.Errorf("version = %q, want %q", got, "1.2.3")
	}

	if len(runner.Calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(runner.Calls))
	}
	call := runner.Calls[0]
	if call.Dir != "/repo" {
		t.Errorf("dir = %q, want %q", call.Dir, "/repo")
	}
	if !reflect.DeepEqual(call.Invocation.Argv, []string{"print-version"}) {
		t.Errorf("argv = %#v, want %#v", call.Invocation.Argv, []string{"print-version"})
	}
	if !reflect.DeepEqual(call.Invocation.Env, []string{"BUMPER_GROUP=api"}) {
		t.Errorf("env = %#v, want %#v", call.Invocation.Env, []string{"BUMPER_GROUP=api"})
	}
}

func TestGetCurrentVersionErrors(t *testing.T) {
	group := ReleaseGroup{
		Name:       "api",
		CurrentCMD: []string{"print-version"},
	}

	t.Run("runner failure", func(t *testing.T) {
		runner := &FakeRunner{Err: errors.New("boom")}
		if _, err := GetCurrentVersion(t.Context(), runner, "/repo", group); err == nil {
			t.Error("expected error when runner fails")
		}
	})

	t.Run("unparsable version", func(t *testing.T) {
		runner := &FakeRunner{Stdout: map[Verb]string{VerbCurrent: "not-a-version\n"}}
		if _, err := GetCurrentVersion(t.Context(), runner, "/repo", group); err == nil {
			t.Error("expected error for unparsable version output")
		}
	})
}

func TestGetNextVersion(t *testing.T) {
	group := ReleaseGroup{
		Name:       "api",
		CurrentCMD: []string{"print-version"},
	}

	tests := []struct {
		name  string
		level BumpLevel
		want  string
	}{
		{"patch", BumpLevelPatch, "1.2.4"},
		{"minor", BumpLevelMinor, "1.3.0"},
		{"major", BumpLevelMajor, "2.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &FakeRunner{Stdout: map[Verb]string{VerbCurrent: "1.2.3\n"}}
			got, err := GetNextVersion(t.Context(), runner, "/repo", group, tt.level)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("next version = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("invalid level", func(t *testing.T) {
		runner := &FakeRunner{Stdout: map[Verb]string{VerbCurrent: "1.2.3\n"}}
		if _, err := GetNextVersion(t.Context(), runner, "/repo", group, BumpLevelNone); err == nil {
			t.Error("expected error for invalid bump level")
		}
	})
}

var _ Runner = ExecRunner{}
var _ Runner = (*FakeRunner)(nil)

func TestExecRunner(t *testing.T) {
	inv := GroupInvocation{
		Verb: VerbCurrent,
		Argv: []string{"sh", "-c", "printf '%s' \"$BUMPER_GROUP\""},
		Env:  []string{"BUMPER_GROUP=api"},
	}

	stdout := new(bytes.Buffer)
	if err := (ExecRunner{}).Run(context.Background(), t.TempDir(), inv, stdout); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stdout.String(); got != "api" {
		t.Errorf("stdout = %q, want %q", got, "api")
	}
}
