# TES-13 remediation status

## Implemented in the shared workspace

- Removed `packages/fsr4/` and the `--fsr41` CLI option. Removed game-directory DLL copies and added a write boundary plus regression test.
- Generated wrappers for AMD, NVIDIA, and Intel use `umu-run` with the selected Proton and require `PROTON_EAC_RUNTIME` to exist. They write the launch log in the prefix.
- Runtime upscaler upgrades remain off by default; the game's shipped upscalers are left alone.
- Vendored winetricks is hash checked and installed to `~/.local/bin` without sudo or self-update. The uninstaller requires a Bellum prefix with Wine markers, refuses root/home/symlink paths, and defaults its destructive prompt to No.
- Installer logs use a private directory and `O_NOFOLLOW`; tar entries and link targets are checked before extraction. The user launcher is installed in `~/.local/bin`; desktop permissions are `0644`.
- SHA-256 pins are implemented for the vendored DXVK, winetricks, and icon files. Proton and launcher downloads require release pins and fail closed while those pins are empty. Launcher verification also requires an approved Authenticode signer and a successful `osslsigncode verify`.
- `go test ./...` and `git diff --check` pass. See `docs/eac-qa.md` for the Linux-mode log check.
- A follow-up fixed literal `\\n` escapes in the Intel/generic `launch_vars.env` writer. Targeted `go test ./pkg/workflow ./pkg/launchers ./pkg/packages` passes with a writable Go cache.
- The WebView2 follow-up adds a nonblocking prefix `flock` for second clicks, keeps umu's session-wait verb, verifies the installed WebView2 runtime after launcher setup, and restricts the prefix, launch variables, and launcher log to private permissions. NVIDIA's NGX updater defaults off; the capability-gated FSR runtime switch uses `1` instead of the removed `4.1.0` path. The QA checklist now covers launcher lifecycle and evidence redaction. Focused package tests and `git diff --check` pass.

## Remaining release blockers

1. Read the approved Proton build, EAC runtime, and AstarteLauncher release pins from TES-3 and TES-9. Populate `pkg/config/versions.go`, add pinned EAC provisioning, and verify the Authenticode signer value and `osslsigncode` output against the approved binary.
2. Run the TES-12 EAC Linux-mode QA check against a real Bellum installation and attach the observed log lines.
   Include a real umu container lifecycle test through launcher self-update and tray Quit; static tests cannot establish whether the container stays alive after a launcher process handoff.
3. Review overlap with TES-10/TES-11 in the shared checkout and update the Paperclip issue/work products. The Paperclip API at `127.0.0.1:3100` refused connections in both heartbeats, so the board status and work products could not be changed.

The installer deliberately fails before installation while the release pins are missing. Do not publish this state as a working release.
