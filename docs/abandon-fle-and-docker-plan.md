# Abandon FLE And Docker, Start A Local Factorio QA Harness

## Summary

Abandon Docker for the default developer workflow. The new project runs a local
Factorio headless process directly, manages it as a normal subprocess, and talks
to it over RCON. No image builds, no Compose files, no Docker socket, and no
container maintenance are required in v0.

Create a Go project named `factorio-mod-qa`. Useful QA logic from the old FLE
fork has been ported into Go or discarded; no Python implementation remains.

## New Runtime Model

```text
Go CLI
  starts local Factorio binary
  passes mods/scenario/save/write-dir paths
  connects over RCON
  calls qa_control_mod remote API
  writes reports/artifacts
```

No Docker responsibilities in v0.

Required CLI shape:

```bash
fmqa run \
  --factorio-bin ~/.factorio \
  --write-dir .fmqa \
  --mods-path /path/to/mods \
  --scenario open_world \
  --run-id local-test

fmqa snapshot \
  --rcon localhost:27000 \
  --out prototype_snapshot.json

fmqa validate \
  --snapshot prototype_snapshot.json
```

The CLI creates local working directories automatically:

```text
.fmqa/
  factorio/
    config/
    mods/
    saves/
    script-output/
  runs/<run_id>/
  reports/
```

## Preserved Fixtures

Copied fixtures from the earlier prototype:

- `fixtures/mods/qa-broken-mod/`
- `fixtures/prototype_snapshots/qa_broken_mod.json`

Not copied into the new runtime:

- `mod_qa_agent/factorio_session.py`
- FLE `ClusterManager`
- FLE `FactorioInstance`
- FLE namespace eval
- FLE `GameState`
- `mod_qa_agent/rcon_probe/`
- Docker Compose files

## Initial Project Structure

```text
factorio-mod-qa/
  go.mod
  cmd/fmqa/main.go
  internal/rcon/
  internal/factorio/
  internal/runner/
  internal/snapshot/
  internal/reports/
  qa_control_mod/
    info.json
    control.lua
  fixtures/
  docs/
    abandon-fle-and-docker-plan.md
    architecture.md
```

## Acceptance Criteria

- No Docker or Compose dependency exists in the new project.
- The CLI can start a local Factorio binary directly.
- The CLI can connect to RCON and export a snapshot.
- Static validation runs without importing `fle`.
- Live run against `qa-broken-mod` detects:
  - missing crafting machine,
  - positive output loop,
  - script event growth from `qa-ticking-machine`.
- Space Age data is never stripped or disabled by default.
- Native Factorio saves are the primary repro artifact.

## Assumptions

- Go is the long-term primary language.
- Go is the only implementation language.
- User provides a local Factorio install path with the required DLC/mod setup.
- Rootless Docker/Podman may be considered later only as optional CI
  infrastructure, not as the default developer workflow.
