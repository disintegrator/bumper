package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
)

// Verb identifies one of the group-command hooks a release group configures in
// .bumper/config.toml.
type Verb string

const (
	VerbCurrent   Verb = "current"
	VerbNext      Verb = "next"
	VerbChangelog Verb = "changelog"
	VerbCat       Verb = "cat"
)

// GroupInvocation is a fully-resolved group command: the argv to execute and
// the BUMPER_* environment variables that carry the protocol inputs. Env holds
// KEY=VALUE pairs appended to the parent process environment.
type GroupInvocation struct {
	Verb Verb
	Argv []string
	Env  []string
}

func NewCurrentInvocation(group ReleaseGroup) (GroupInvocation, error) {
	if len(group.CurrentCMD) == 0 {
		return GroupInvocation{}, errors.New("no current version command defined for release group")
	}

	return GroupInvocation{
		Verb: VerbCurrent,
		Argv: slices.Clone(group.CurrentCMD),
		Env: []string{
			fmt.Sprintf("BUMPER_GROUP=%s", group.Name),
		},
	}, nil
}

func NewNextInvocation(group ReleaseGroup, nextVersion string) (GroupInvocation, error) {
	if len(group.NextCMD) == 0 {
		return GroupInvocation{}, errors.New("no next version command defined for release group")
	}

	return GroupInvocation{
		Verb: VerbNext,
		Argv: slices.Clone(group.NextCMD),
		Env: []string{
			fmt.Sprintf("BUMPER_GROUP=%s", group.Name),
			fmt.Sprintf("BUMPER_GROUP_NEXT_VERSION=%s", nextVersion),
		},
	}, nil
}

func NewChangelogInvocation(group ReleaseGroup, nextVersion string, status *ReleaseGroupStatus) (GroupInvocation, error) {
	if len(group.ChangelogCMD) == 0 {
		return GroupInvocation{}, errors.New("no changelog command defined for release group")
	}

	argv := slices.Clone(group.ChangelogCMD)
	argv = append(argv, "--group", group.Name)
	for _, entry := range status.MajorLogs {
		argv = append(argv, "--major", entry.Content)
	}
	for _, entry := range status.MinorLogs {
		argv = append(argv, "--minor", entry.Content)
	}
	for _, entry := range status.PatchLogs {
		argv = append(argv, "--patch", entry.Content)
	}

	return GroupInvocation{
		Verb: VerbChangelog,
		Argv: argv,
		Env: []string{
			fmt.Sprintf("BUMPER_GROUP=%s", group.Name),
			fmt.Sprintf("BUMPER_GROUP_NEXT_VERSION=%s", nextVersion),
		},
	}, nil
}

func NewCatInvocation(group ReleaseGroup, version string) (GroupInvocation, error) {
	if len(group.CatCMD) == 0 {
		return GroupInvocation{}, errors.New("no cat command defined for release group")
	}

	return GroupInvocation{
		Verb: VerbCat,
		Argv: slices.Clone(group.CatCMD),
		Env: []string{
			fmt.Sprintf("BUMPER_GROUP=%s", group.Name),
			fmt.Sprintf("BUMPER_GROUP_VERSION=%s", version),
		},
	}, nil
}

// Runner executes resolved group commands. ExecRunner is the process-spawning
// implementation; FakeRunner substitutes in tests.
type Runner interface {
	Run(ctx context.Context, dir string, inv GroupInvocation, stdout io.Writer) error
}

// ExecRunner runs group commands as child processes with stderr passed through
// to the parent process.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, inv GroupInvocation, stdout io.Writer) error {
	cmd := exec.CommandContext(ctx, inv.Argv[0], inv.Argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), inv.Env...)
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// FakeRunner records every invocation and plays back canned stdout per verb so
// callers can be tested without spawning processes.
type FakeRunner struct {
	Calls  []FakeRunnerCall
	Stdout map[Verb]string
	Err    error
}

type FakeRunnerCall struct {
	Dir        string
	Invocation GroupInvocation
}

func (f *FakeRunner) Run(_ context.Context, dir string, inv GroupInvocation, stdout io.Writer) error {
	f.Calls = append(f.Calls, FakeRunnerCall{Dir: dir, Invocation: inv})
	if f.Err != nil {
		return f.Err
	}

	if out, ok := f.Stdout[inv.Verb]; ok {
		if _, err := io.WriteString(stdout, out); err != nil {
			return err
		}
	}

	return nil
}
