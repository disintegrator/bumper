#!/usr/bin/env sh

#MISE dir="{{ cwd }}"

set -e

export MISE_QUIET=true

go build -o "$MISE_CONFIG_ROOT/local/bumper" "$MISE_CONFIG_ROOT/cmd/bumper/main.go"

exec "$MISE_CONFIG_ROOT/local/bumper" "$@"