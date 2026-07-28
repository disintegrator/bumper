# Changelog

## Bumper 0.1.0

### Minor Changes

- 637dc8c: Added `current:toml`, `next:toml`, `current:json`, `next:json`, `current:yaml` and `next:yaml` builtins that read and update a version string at a dot-separated key path (`--key`) in TOML, JSON and YAML files (`--path`), preserving the surrounding formatting and comments. Useful for manifests like `pyproject.toml`, `Cargo.toml`, `composer.json` or `galaxy.yml`.

## Bumper 0.0.7

### Patch Changes

- dfd535f: Fixed two crashes in bump provenance resolution: running `bumper commit` or `bumper next` in a directory that is not a git repository now completes with a warning (changelog entries are written without commit prefixes), and a bump file introduced in the repository's root commit now resolves to that commit. The shallow-clone deepening loop also no longer re-walks bump files that were already resolved.
- 768e8e5: `bumper bump` no longer requires a terminal when all inputs are provided via flags. Invocations like `bumper bump --group api --minor -m "msg"` now work in CI and other non-interactive environments; the interactive form only runs when a group, level, or message still needs to be collected.
- 91ec162: Commands now report clear errors where they previously exited silently: an unknown `--group`, a missing `--group` with multiple release groups defined, and a missing or invalid configuration each print a descriptive message. `bumper init` also defaults to the current directory as documented.
- 49fdf3a: `bumper builtins cat:default` now defaults to `CHANGELOG.md` in the workspace root, matching `amendlog:default`. Previously it resolved the default path relative to the process working directory, so reading release notes from a subdirectory of the workspace failed to find the changelog that amending had written at the root.
- b2dce5a: `bumper commit` no longer deletes pending bump files before running release group commands. Bump files are now removed only after every group's version and changelog commands have succeeded, so a failure partway through a multi-group release preserves the release intent and the commit can be retried. Failures also report which release group failed.
- 145ff63: Retrying `bumper commit` after a partial failure no longer re-releases groups that already succeeded. Commit progress is checkpointed to `.bumper/commit-checkpoint.toml` after each group completes; a retry skips checkpointed groups, so each group's version bump and changelog amendment run exactly once per batch of pending bumps. The checkpoint is removed when the batch completes, and modifying the pending bump files between attempts starts the batch from scratch.
- 68cc203: Bump files with CRLF line endings or without a trailing newline now parse correctly. Previously a Windows-authored bump file failed with "front matter must start with ---", and a file whose closing `---` lacked a trailing newline corrupted the front matter.

## Bumper 0.0.6

### Patch Changes

- 7ecb5a4: Updated the commit command to gradually unshallow a git repo until the initial commit for each bump file is found. This ensures that the history of changes to these files is accurately tracked. This is a best effort approach and any bump files that cannot be resolved will be logged as warnings.

## Bumper 0.0.5

### Patch Changes

- bc35fd2: Added `bumper next` command which will display the next version of a release group based on the pending bumps in the workspace.
- bc35fd2: Bumper commands `bump`, `cat`, `current` and `next` now validate that at least one group is defined in the configuration file before proceeding.
- bc35fd2: Rename `bumper latest` to `bumper current`
- bc35fd2: Make the --group flag optional for several commands when there is only one release group

## Bumper 0.0.4

### Patch Changes

- 13c6ee8: Run all commands relative to workspace directory

## Bumper 0.0.3

### Patch Changes

- e90fdd8: Only show release group selector if there are more than one release groups

## Bumper 0.0.2

### Patch Changes

- 8b11c01: Provide apk, dev, rpm and arch packages

## Bumper 0.0.1

### Patch Changes

- d9e9c7c: Initial release of `bumper`
