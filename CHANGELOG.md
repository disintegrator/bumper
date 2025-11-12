# Changelog

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
