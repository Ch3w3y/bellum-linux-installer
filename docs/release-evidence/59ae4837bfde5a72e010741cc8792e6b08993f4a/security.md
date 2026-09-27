# Security review: v2.2.0 (candidate v2.2.0-rc.7)

- **Candidate:** `v2.2.0-rc.7`, commit `59ae4837bfde5a72e010741cc8792e6b08993f4a`
- **Reviewed:** 2026-09-27, by Claude (AI coding assistant) for the maintainer; approval rests with the maintainer on the `release` environment.
- **Scope:** the whole tree at the candidate, with focus on the changes since the September board review (PRs #28–#35) and on every path that downloads, verifies, executes or deletes.

## Automated checks on the candidate

| Check | Result | Where |
| --- | --- | --- |
| `make check` (gofmt, `go vet`, `go test ./...`, `go mod verify`, tidy drift) | passed, amd64 and arm64 | [release run 36332615564](https://github.com/Ch3w3y/bellum-linux-installer/actions/runs/36332615564) |
| `govulncheck ./...` (v1.1.4, Go 1.26.8) | "No vulnerabilities found." | same run, "Code and vulnerability checks" |
| Release build + `make verify-release` | passed; archives attested (SLSA provenance, Sigstore) | same run |
| `scripts/test-release.sh` (gate, dirty tree, tampering, asset set) | passed | local run at the candidate commit, 2026-09-27 |
| `shellcheck install.sh` and the CI `install.sh` dry run | passed | CI on #35 |

## Manual review

**Secrets and personal data**
- Searched the tree and the full Git history (all branches) for GitHub, AWS and Slack token formats, private keys and passwords: none found.
- The tree contains no personal names, e-mail addresses, hostnames or home paths beyond placeholders; the screenshot hostname was neutralised in #35.
- Commit metadata in the history includes contributors' e-mail addresses; this is standard Git metadata and was left to the maintainer's decision.

**Downloads and verification**
- Proton-CachyOS and umu-launcher are pinned by SHA-256 and fail closed (see the provenance record).
- Proton extraction rejects traversal, absolute paths, links that leave the tree or sit under another link, and skips device entries; this was tested against the real 2,068-link archive (#29).
- The Astarte Launcher installer and launcher updates require an Authenticode signature with exactly the signer `ASTARTE INDUSTRIES INC.`, chained to the system roots plus the pinned GlobalSign Code Signing Root R45.
- Launcher updates accept only the official `releases.astarte.industries/astartelauncher/windows-amd64/<version>/AstarteLauncher.exe` URL for a stable version from the release list, over HTTPS (#32).
- `install.sh` verifies the archive against the release `SHA256SUMS` and every file inside against the archive's own checksums. Releases carry build-provenance attestations for independent verification.

**Execution**
- Every external command is run with a fixed argv and no shell.
- The wrapper and `launch_vars.env` values are single-quote escaped.
- The installer, uninstaller and `update-launcher` refuse root.
- `sudo` is used only by `install.sh`, only for a missing `python3` or `flock`, and only after a `[Y/n]` prompt.

**Deletes**
- All recursive prefix deletes go through `removeBellumPrefix` (#31). It requires an absolute path named `Bellum` that is not `/`, home, a home ancestor or inside a system folder; no symlinks and the current user's ownership; the ownership manifest for that exact path; the expected contents; and no mounted filesystem inside.
- Tests cover `/`, home and `$HOME/..` end to end.

**Input hardening (#35)**
- `install.sh` accepts only `X.Y.Z` or `X.Y.Z-rc.N` versions. Before, `v..` passed validation and could have made the unpack step replace `~/.local/share` with a hostile mirror.
- The install path rejects control characters, which could have injected fields into the `.desktop` file.

**Other fixes**
- A read-only `/dev/null` made silent commands fail (#30).
- The release gate could never pass (#33).

**CI/CD**
- Every Action is pinned to a commit SHA, with minimal permissions per workflow.
- Workflow inputs reach scripts only through `env:`, and there is no `pull_request_target`.
- `delete-branch` refuses the default branch.
- Final releases need the evidence gate and approval on the `release` environment, which the maintainer has enabled.

## Residual risks (accepted)

- Authenticode verification has no revocation check.
- An unknown launcher or EAC-runtime digest only warns, because both are updated by their vendors; the signer and the file set remain hard requirements.
- `install.sh`'s own checksum check is integrity, not authenticity (the checksums come from the same release). Authenticity comes from HTTPS to GitHub and the attestations, which users can verify as documented in the README.
- The original installer's code (about 22% of lines) was published without a license; the README and CREDITS say so pending its author's agreement.

**Conclusion:** no open high or medium findings at the candidate.
