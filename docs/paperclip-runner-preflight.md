> **Internal Paperclip/DevOps infrastructure record, not Bellum installer
> documentation.** This file documents the Paperclip agent runner's own
> adapter/preflight behavior on the machine running these agents — it has no
> connection to the Bellum game, the installer/uninstaller binaries, or any
> end user of this repository. See TES-42's `review` document for a
> recommendation to move this file out of the user-facing `docs/` tree.

# Paperclip managed-run API preflight + durable dist patches (TES-21)

State: implemented, verified locally, and verified against a live service restart (2026-09-26). Owner: DevOps.

## What changed

1. **Authenticated preflight before issue work** — `@paperclipai/adapter-opencode-local`'s
   `execute()` now runs a strict-2xx authenticated `GET <base>/api/health` before the
   provider process spawns (before `opencode models` and before any token spend):
   - base URL normalized: `http://host:3100`, `http://host:3100/`, and
     `http://host:3100/api/` all resolve to `<base>/api/health`
   - requires HTTP 2xx; 401/404/5xx/connection failure all fail the gate
   - bounded tries (default 3 × 5s timeout, 1s backoff; tunable via
     `PAPERCLIP_PREFLIGHT_TRIES` / `PAPERCLIP_PREFLIGHT_TIMEOUT_SEC`)
   - audit header `X-Paperclip-Run-Id` sent on the probe
2. **Bounded reschedule on failure** — a failed preflight returns
   `errorCode=paperclip_api_preflight_failed` with
   `errorFamily=transient_upstream`, which flows through the server's existing
   `readTransientRecoveryContractFromRun` → `scheduleBoundedRetryForRun` path
   (2 retries, 30s apart). No provider tokens are consumed, no issue state is
   stranded (nothing was written; the issue execution lock is released by the
   normal failure path), and the run log shows the preflight error.
3. **Durable patches after upgrades** — `~/.paperclip/patches/paperclip_adapter_patches.py`
   applies and verifies BOTH local dist patches idempotently:
   - `codex-acp-network-access` (TES-20): sandboxed AgentMode presets honor
     `PAPERCLIP_RUNNER_NETWORK_ACCESS=enabled`
   - `opencode-local-api-preflight` (TES-21): the preflight above
   A systemd drop-in (`~/.config/systemd/user/paperclipai.service.d/paperclip-patches.conf`)
   runs `apply` as `ExecStartPre` before every server start, so
   `paperclipai update` → dist replaced → restart → patches re-applied
   automatically. Patch-shape drift fails the start loudly (non-zero exit)
   instead of silently degrading.

## Files

- `~/.paperclip/patches/paperclip_adapter_patches.py` — maintained patch runner (apply|status|verify)
- `~/.paperclip/patches/paperclip-api-preflight.sh` — standalone preflight (canonical; workspace copy in `scripts/`)
- `~/.paperclip/patches/codex_acp_network_patch_main.py` — TES-20 single patch (superseded by the runner, still works standalone)
- `~/.config/systemd/user/paperclipai.service.d/paperclip-patches.conf` — auto re-apply hook

## Deployment steps (this instance)

Already applied. For a fresh instance:

```
python3 ~/.paperclip/patches/paperclip_adapter_patches.py apply
systemctl --user daemon-reload
systemctl --user restart paperclipai.service
python3 ~/.paperclip/patches/paperclip_adapter_patches.py verify   # → all applied
```

## Tests performed

- preflight script: 200 OK (1 try), trailing-slash and `/api/` base normalization, 401 → rc=1, connection refused → rc=1
- patch runner: apply (fresh), re-apply (idempotent), verify, simulated upgrade wipe of BOTH dist files → `apply` restored both; module imports cleanly post-patch
- generated preflight logic: live 200 pass; 401 reject; conn-fail reject; missing-env skip; normalization (same as script tests)

## Live restart verification (2026-09-26 16:11:29 BST)

Real systemd restart of `paperclipai.service` proved the ExecStartPre hook end-to-end:

```
Sep 26 16:11:29 omarchy-vm python3[518063]: codex-acp-network-access: already applied
Sep 26 16:11:29 omarchy-vm python3[518063]: opencode-local-api-preflight: already applied
Sep 26 16:11:30 omarchy-vm paperclipai[518066]: ┌   paperclipai run
```

- `systemctl --user show paperclipai.service` after restart: `ActiveState=active`, `SubState=running`, `ExecMainStatus=0`, `ExecMainStartTimestamp=Sat 2026-09-26 16:11:29 BST`.
- Post-restart patch state: `status` shows both patches applied under `/home/daryn/.paperclip/cli/current/...`; `verify` exits 0.
- All runs after the restart (including the artifact upload in this issue) performed authenticated GETs and audited comment/status writes without escalation — the acceptance criterion for a normal sandbox run.

Journal evidence is attached to TES-21 as
`paperclipai.service restart verification (ExecStartPre patch evidence)`
(attachment `1b73d2b8-4d3a-4916-9f72-6874541418ac`).

## Remaining upstream dependency

The durable, upgrade-free fix is to honor a controller-projected network knob in
`@agentclientprotocol/codex-acp` sandboxed presets (upstream), and to add the
preflight gate to `@paperclipai/adapter-*` upstream. Until then the
ExecStartPre patcher is the maintained path; if an upstream release changes the
patch anchors, `apply` fails loudly at startup and the anchors must be re-derived.
