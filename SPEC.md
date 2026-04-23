# Prerelease Support Specification

This document describes the design for adding prerelease version support to Bumper.

## Overview

Prereleases allow release groups to publish unstable versions (alpha, beta, rc, etc.) before graduating to a stable release. This feature integrates seamlessly with existing bump workflows and CI/CD processes.

## Commands

### `bumper pre enter <group> --tag <tag>`

Enters prerelease mode for a release group.

**Arguments:**
- `<group>`: The release group name
- `--tag <tag>`: The prerelease tag (e.g., `alpha`, `beta`, `rc`)

**Behavior:**
- Validates the group exists and is not already in prerelease mode
- Creates/updates `.bumper/prerelease.toml` with the group's prerelease state
- Does not commit or release anything — purely a state change
- Pending bumps (if any) will be processed on the next `bumper commit`

**Example:**
```sh
bumper pre enter dashboard --tag alpha
# Output: Entered prerelease for 'dashboard' with tag 'alpha'
#         Next commit will produce: 1.3.0-alpha.1
```

### `bumper pre exit <group>`

Exits prerelease mode and graduates to a stable release.

**Arguments:**
- `<group>`: The release group name

**Behavior:**
1. Validates the group is in prerelease mode
2. Reads all processed bumps from `.bumper/prerelease/`
3. Reads any pending bumps from `.bumper/bump-*.md`
4. Consolidates all notes into a single stable changelog entry
5. Updates version to stable (e.g., `1.3.0-rc.2` → `1.3.0`)
6. Deletes all bump files (both pending and prerelease)
7. Removes the group from `.bumper/prerelease.toml`

**Example:**
```sh
bumper pre exit dashboard
# Output: Exited prerelease for 'dashboard'
#         Released version: 1.3.0
```

### `bumper pre status [group]`

Shows the current prerelease state.

**Arguments:**
- `[group]`: Optional. If provided, shows status for that group only.

**Example:**
```sh
bumper pre status
# Output:
# dashboard: 1.3.0-alpha.2 (tag: alpha, from: 1.2.3)
# api: not in prerelease
```

## State Management

### Prerelease State File

Prerelease state is tracked in `.bumper/prerelease.toml`:

```toml
[groups.dashboard]
tag = "alpha"
from_version = "1.2.3"
counter = 2
```

| Field | Description |
|-------|-------------|
| `tag` | The prerelease identifier (alpha, beta, rc, etc.) |
| `from_version` | The stable version when prerelease was entered |
| `counter` | The current prerelease number (1, 2, 3, ...) |

This file should be committed to version control.

### Bump File Lifecycle

During prerelease, bump files follow a different lifecycle:

```
.bumper/
├── config.toml
├── prerelease.toml           # prerelease state
├── bump-*.md                 # pending bumps (not yet released)
└── prerelease/
    └── bump-*.md             # processed bumps (released as prerelease)
```

**Normal (stable) flow:**
1. Bump files created in `.bumper/bump-*.md`
2. `bumper commit` processes and deletes them

**Prerelease flow:**
1. Bump files created in `.bumper/bump-*.md`
2. `bumper commit` processes and moves them to `.bumper/prerelease/`
3. On `pre exit`, all files in `.bumper/prerelease/` are consolidated and deleted

This design enables CI/CD triggers based on file changes:
- Watch `.bumper/bump-*.md` for new pending changes
- Watch `.bumper/prerelease/` for prerelease activity

## Version Calculation

### Bump Levels Dictate Version

During prerelease, bump levels continue to dictate the target version rather than naively incrementing the prerelease counter.

**Algorithm:**

1. Get `from_version` from prerelease state (e.g., `1.2.3`)
2. Determine accumulated level from bumps in `.bumper/prerelease/`
3. Determine pending level from bumps in `.bumper/bump-*.md`
4. Take the highest of accumulated and pending levels
5. Calculate base version: `from_version` + highest level
6. If base version changed from previous prerelease → reset counter to 1
7. Else → increment counter

**Examples:**

| Current | Pending Bump | Accumulated | Result |
|---------|--------------|-------------|--------|
| 1.2.3 (stable) | minor | - | 1.3.0-alpha.1 |
| 1.3.0-alpha.1 | patch | minor | 1.3.0-alpha.2 (patch < minor) |
| 1.3.0-alpha.2 | major | minor | 2.0.0-alpha.1 (major > minor, escalates) |
| 2.0.0-alpha.1 | minor | major | 2.0.0-alpha.2 (minor < major) |

### Tag Progression

Changing tags resets the counter but preserves the base version:

```sh
bumper commit                           # 1.3.0-alpha.1
bumper commit                           # 1.3.0-alpha.2
bumper pre enter dashboard --tag beta   # switch to beta (or: bumper pre retag)
bumper commit                           # 1.3.0-beta.1
bumper pre enter dashboard --tag rc
bumper commit                           # 1.3.0-rc.1
bumper pre exit dashboard               # 1.3.0
```

## Changelog Behavior

### Append-Only Design

The changelog remains append-only. Prerelease entries are written during the prerelease phase and remain in history.

**During prerelease (after several commits):**

```markdown
## 1.3.0-alpha.2
- Fixed bug in feature X

## 1.3.0-alpha.1
- Added feature X
- Refactored Y

## 1.2.3
- Previous stable release
```

### Consolidated Stable Entry

On `pre exit`, a consolidated stable entry is prepended containing all changes from the prerelease cycle:

```markdown
## 1.3.0
- Added feature X
- Refactored Y
- Fixed bug in feature X
- Final polish   ← from pending bumps at exit time

## 1.3.0-alpha.2
- Fixed bug in feature X

## 1.3.0-alpha.1
- Added feature X
- Refactored Y

## 1.2.3
- Previous stable release
```

This provides:
- **Full history** for debugging and auditing (prerelease entries preserved)
- **Consolidated view** for stable release consumers (1.3.0 entry has everything)

## `bumper commit` Behavior

### No-Op with No Pending Bumps

`bumper commit` is a no-op when there are no pending bumps. This applies to both stable and prerelease modes.

### Modified Behavior in Prerelease Mode

When a group is in prerelease mode, `bumper commit`:

1. Reads pending bumps from `.bumper/bump-*.md`
2. If no pending bumps → no-op (exit early)
3. Reads processed bumps from `.bumper/prerelease/` for accumulated level
4. Calculates next prerelease version (see Version Calculation)
5. Updates version using `next_cmd`
6. Writes changelog entry using `changelog_cmd`
7. Moves pending bump files to `.bumper/prerelease/`
8. Updates `counter` in `prerelease.toml`

## CI/CD Integration

### Workflow Examples

**Standard release workflow (handles both stable and prerelease):**

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    branches: [main]
    paths:
      - '.bumper/bump-*.md'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Release
        run: bumper commit --group myapp
        # Automatically produces prerelease or stable based on prerelease.toml
```

**Enter prerelease (manual trigger):**

```yaml
# .github/workflows/prerelease-enter.yml
name: Enter Prerelease
on:
  workflow_dispatch:
    inputs:
      group:
        description: 'Release group'
        required: true
      tag:
        description: 'Prerelease tag'
        required: true
        type: choice
        options: [alpha, beta, rc]

jobs:
  enter:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Enter prerelease
        run: |
          bumper pre enter ${{ inputs.group }} --tag ${{ inputs.tag }}
          git add .bumper/prerelease.toml
          git commit -m "chore: enter ${{ inputs.tag }} prerelease for ${{ inputs.group }}"
          git push
```

**Exit prerelease (manual trigger):**

```yaml
# .github/workflows/prerelease-exit.yml
name: Graduate to Stable
on:
  workflow_dispatch:
    inputs:
      group:
        description: 'Release group'
        required: true

jobs:
  graduate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Graduate to stable
        run: |
          bumper pre exit ${{ inputs.group }}
          git add -A
          git commit -m "chore: release stable ${{ inputs.group }}"
          git push
```

### File-Based Triggers

The bump file design enables granular CI triggers:

```yaml
# Trigger on new pending bumps
on:
  push:
    paths:
      - '.bumper/bump-*.md'

# Trigger on prerelease activity
on:
  push:
    paths:
      - '.bumper/prerelease/**'

# Trigger on prerelease state changes
on:
  push:
    paths:
      - '.bumper/prerelease.toml'
```

## Edge Cases

### Entering Prerelease with No Pending Bumps

Allowed. The group enters prerelease mode but no version is released until the next `bumper commit` with pending bumps.

### Exiting Prerelease with No Processed Bumps

This would mean `pre enter` was called but no commits were made. `pre exit` should:
- Warn the user
- Clean up state (remove from prerelease.toml)
- Not write a changelog entry or update version

### Multiple Groups in Prerelease

Each group can independently be in prerelease mode with different tags:

```toml
# .bumper/prerelease.toml
[groups.dashboard]
tag = "beta"
from_version = "2.0.0"
counter = 3

[groups.api]
tag = "alpha"
from_version = "1.5.0"
counter = 1
```

### Switching Tags Mid-Prerelease

Use `pre enter` with a new tag (or a dedicated `pre retag` command):

```sh
bumper pre enter dashboard --tag beta  # already in alpha
# Keeps from_version, resets counter, changes tag
```

## Summary

| Aspect | Design Decision |
|--------|-----------------|
| Commands | `pre enter`, `pre exit`, `pre status` |
| State | `.bumper/prerelease.toml` (minimal: tag, from_version, counter) |
| Bump files | Moved to `.bumper/prerelease/` on commit, deleted on exit |
| Version calc | Bump levels dictate version, not naive counter increment |
| Changelog | Append-only; stable entry consolidates all prerelease changes |
| No pending bumps | `bumper commit` is a no-op |
| CI/CD | File-based triggers, no flags needed on commit |
