#!/usr/bin/env bash
# Select the next patch from stable tags; reuse a tag when retrying its commit.
set -euo pipefail

latest=$(git tag --list --sort=-version:refname | sed -nE '/^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/p' | sed -n '1p')
if [ -z "$latest" ]; then
  echo 'No stable vX.Y.Z tag exists; create the initial release manually.' >&2
  exit 1
fi
if [ "$(git rev-parse "$latest^{commit}")" = "$(git rev-parse HEAD)" ]; then
  printf '%s\n' "$latest"
else
  prefix=${latest%.*}
  patch=${latest##*.}
  printf '%s.%s\n' "$prefix" "$((patch + 1))"
fi
