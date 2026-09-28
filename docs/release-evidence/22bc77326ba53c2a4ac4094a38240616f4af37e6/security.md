# Security review: v2.3.0 (candidate v2.3.0-rc.1)

- **Candidate:** `v2.3.0-rc.1`, commit `22bc77326ba53c2a4ac4094a38240616f4af37e6`
- **Reviewed:** 2026-09-28, by Claude (AI coding assistant) for the maintainer; approval rests with the maintainer on the `release` environment.
- **Scope:** everything since v2.2.0 (#41, #42, #43): platform detection, launch profiles, NVIDIA prechecks, Snap Steam, the Steam shortcut writer, wrapper changes and the uninstaller. Paths that download, verify, extract or delete are unchanged since the v2.2.0 review (`docs/release-evidence/59ae4837bfde5a72e010741cc8792e6b08993f4a/security.md`).

## Automated checks on the candidate

| Check | Result | Where |
| --- | --- | --- |
| `make check` (gofmt, `go vet`, `go test ./...`, `go mod verify`, tidy drift) | passed, amd64 and arm64 | [release run 36441431798](https://github.com/Ch3w3y/bellum-linux-installer/actions/runs/36441431798), "Code and vulnerability checks" |
| `govulncheck ./...` (v1.1.4, Go 1.26.8) | passed (the step fails on any finding) | same run and step, both architectures |
| PR CI: gofmt/vet/test, both builds, `install.sh` shellcheck and dry run | passed | [run 36441093449](https://github.com/Ch3w3y/bellum-linux-installer/actions/runs/36441093449) on #43 |
| `go test -race ./pkg/...` | passed | local, 2026-09-28 |
| Fuzzing `steamvdf.Parse` (every input either rejected as malformed or round-tripped byte for byte) | ~5.8 million executions, no failure | local, 2026-09-28 |
| `scripts/test-release.sh` (gate, dirty tree, tampering, asset set) | passed | local, 2026-09-28 |
| An independent AI code review of the whole v2.3 diff | 10 findings; all fixed in #43 except one efficiency note (a double read of `shortcuts.vdf` in the opt-in prompt) | commit `13ce01a` |

No module dependency was added; `go.mod` and `go.sum` are unchanged since v2.2.0.

## Manual review

**New reads (platform detection).** Everything is read through a size-bounded adapter (1 MiB text, 64 KiB os-release, 8 KiB sysfs/proc values), and directory scans are capped at 4,096 entries. Symlinks are resolved with loop and escape checks. Untrusted strings (os-release, DMI, renderer, driver banner) are stripped of control characters and terminal escapes before display. Detection never reads command lines, serial numbers or Bluetooth addresses, and never sends anything off the machine. GPU probes run `glxinfo -B` and `lspci -nn` with fixed argv and per-probe timeouts (4 s and 2 s).

**New write: Steam's `shortcuts.vdf` (opt-in, default No).**
- It is never answered by `--yes`, and it only targets native Steam's single signed-in (or most recent) account.
- Steam must be closed, checked through `/proc`. Unknown counts as running, and the check is repeated just before the rename.
- It refuses a symlinked config folder or file, a file owned by another user, and any content the strict parser rejects.
- The parser allows five node types and fails on truncated or trailing data. It caps the file at 4 MiB, nesting at 16 levels and the node count at 200,000.
- The original is backed up with `O_EXCL`. The new file is encoded, re-parsed, written to a temporary file in the same folder with the original mode, fsynced and renamed.
- The uninstaller removes only entries matching both the Bellum wrapper path and the name `Bellum`, with the same checks.

**Commands shown, never run.** NVIDIA and dependency fixes are constant strings per distro; only a sanitized driver version is interpolated. The installer never executes them.

**Wrapper.** The new checks use `${SteamGameId:-}`, `${GAMESCOPE_WAYLAND_DISPLAY:-}` and `pgrep -u "$(id -u)" -x steam`, and only append fixed text to `launcher.log`. The script passes `bash -n`, and the wrapper tests run it in bash.

**Launch settings.** RDNA2 and older AMD GPUs lose `DXIL_SPIRV_CONFIG`, and no new `PROTON_*` variable is written (enforced by `TestLaunchVarsOnlyUseContractedProtonVariables`). The Proton contract now also pins the bundled DXVK major (3.x) that the NVIDIA minimum driver depends on.

## Residual risks (accepted)

- Steam could start between the final running check and the rename, then overwrite `shortcuts.vdf` from memory on exit; the backup keeps the original.
- The v2.2.0 residual risks still apply unchanged: no Authenticode revocation check, warn-only vendor digests, and `install.sh` integrity relying on HTTPS plus attestations.

**Conclusion:** no open high or medium findings at the candidate.
