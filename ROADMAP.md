# Labcoat Roadmap

## Phase 1 — Stability & UX Polish

| # | Item | Rationale |
|---|------|-----------|
| 1.1 | ~~**Fix SSH trust lockup (#34)**~~ | ✅ Done — SSH pre-flight check with inline diagnostics, in-flight guard, and exec error classification. |
| 1.2 | ~~**Persist deployment history per host**~~ | ✅ Done — Deployment records (timestamp, fingerprint, success/failure) persisted per host in BoltDB. `deployments` command added to palette. |
| 1.3 | **Ping-based reboot tracking** | Half-finished feature in README checklist. After issuing a reboot, poll with `ping` / `ssh` to detect when the host comes back, then auto-refresh status. |
| 1.4 | **Better error surfacing** | Extend modal dialogs to cover runner failures, flake parse errors, etc. so the user never needs to check a separate log. (SSH error surfacing done in #34.) |

## Phase 2 — Deployment Intelligence

| # | Item | Rationale |
|---|------|-----------|
| 2.1 | **Configurable host commands with optional confirmation** | Extend the run-cmd infrastructure (already has history + dialog) to allow user-defined commands per-host in `config.toml`, e.g. `drain kubernetes`, `push to cache`. |
| 2.2 | **Gather deployment/generation state from targets** | Unchecked README feature. Run `nixos-version` or read `/run/current-system` on each host to determine the active generation. |
| 2.3 | **Flag out-of-date hosts in list UI** | Depends on #2.2. Compare the host's running generation against the current flake fingerprint to visually flag stale hosts. |
| 2.4 | **`--add-root` support (#17)** | Prevents GC from invalidating builds. Store one GC root per host, clean up old ones. Particularly important for labs with non-free software or large closures. |
| 2.5 | **Run commands locally (#18)** | Allow running host-specific commands on the *local* machine (e.g. push to binary cache). Could share the same configurable-command system from #2.1. |

## Phase 3 — Workflow

| # | Item | Rationale |
|---|------|-----------|
| 3.1 | **Deploy dry-run / build-only mode** | Add a "build only" action that runs `nixos-rebuild build` or `nix build` without switching, so users can verify all configs compile before deploying. |
| 3.2 | **Deployment rollback** | Use `nixos-rebuild switch --rollback` or `nix-env --rollback` to offer one-touch rollback to the previous generation. |
| 3.3 | **Health checks / custom probes** | User-definable post-deploy verification commands (e.g. "is nginx responding?"). Could build on the configurable commands system from #2.1. |
