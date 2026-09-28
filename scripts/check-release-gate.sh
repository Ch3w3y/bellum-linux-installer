#!/usr/bin/env bash
# Usage: check-release-gate.sh RELEASE_COMMIT [GATE_FILE]
#
# The evidence in GATE_FILE is gathered on a release candidate (an -rc tag).
# Recording it is itself a commit, so the gate names the tested candidate
# ("Candidate: <sha>") and proves that the released commit is that candidate
# plus only the gate file and evidence records under docs/release-evidence/.
set -euo pipefail

commit=${1:?release commit required}
gate=${2:-docs/release-gate.md}
fail() { echo "$*" >&2; exit 1; }
[[ $commit =~ ^[0-9a-f]{40}$ ]] || fail 'Release commit must be a full commit SHA'

candidate=$(sed -n 's/^Candidate: \([0-9a-f]\{40\}\)$/\1/p' "$gate")
[[ -n $candidate ]] || fail 'Candidate commit missing: add "Candidate: <full SHA of the tested -rc commit>"'
git cat-file -e "$candidate^{commit}" 2>/dev/null || fail "Candidate $candidate is not in this repository's history (fetch full history)"
git merge-base --is-ancestor "$candidate" "$commit" || fail "Candidate $candidate is not an ancestor of $commit"
changed=$(git diff --name-only "$candidate" "$commit" -- . ":(exclude)$gate" ':(exclude)docs/release-evidence/**')
[[ -z $changed ]] || fail "Code changed since the tested candidate $candidate:
$changed"

for label in QA EAC Security Provenance; do
  line=$(grep -E "^${label}: " "$gate" || true)
  [[ $line =~ ^${label}:\ \[x\]\ (https://[^[:space:]]+)$ ]] || fail "$label evidence missing"
  url=${BASH_REMATCH[1]}
  [[ $url != *example.invalid* && $url == *"$candidate"* ]] || fail "$label evidence must identify the candidate commit $candidate"
done
echo "Release gate passed: $commit is candidate $candidate plus evidence records"
