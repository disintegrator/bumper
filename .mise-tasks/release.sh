#!/usr/bin/env sh

#MISE dir="{{ config_root }}"

set -e

export MISE_QUIET=true
export MISE_TASK_OUTPUT=quiet

export BUMPER_GROUP="bumper"

BUMPER_GROUP_VERSION="$(mise run bumper current)"
export BUMPER_GROUP_VERSION

export GORELEASER_CURRENT_TAG=v"$BUMPER_GROUP_VERSION"

notes=$(mise run bumper cat)

echo "$BUMPER_GROUP $GORELEASER_CURRENT_TAG Release Notes:"
echo ""
echo "$notes"