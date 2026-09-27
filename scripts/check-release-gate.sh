#!/usr/bin/env bash
set -euo pipefail

commit=${1:?candidate commit required}
gate=${2:-RELEASE-GATE.md}
[[ $commit =~ ^[0-9a-f]{40}$ ]] || { echo 'Candidate must be a full commit SHA' >&2; exit 1; }
for label in QA EAC Security Provenance; do
  line=$(grep -E "^${label}: " "$gate" || true)
  [[ $line =~ ^${label}:\ \[x\]\ (https://[^[:space:]]+)$ ]] || { echo "$label evidence missing" >&2; exit 1; }
  url=${BASH_REMATCH[1]}
  [[ $url != *example.invalid* && $url == *"$commit"* ]] || { echo "$label evidence must identify this candidate commit" >&2; exit 1; }
done
