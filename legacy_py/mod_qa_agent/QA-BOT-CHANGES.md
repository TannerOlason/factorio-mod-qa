# Legacy Python QA Changes

This file tracks changes specific to the temporary Python bootstrap copy inside
`factorio-mod-qa`.

## Current Slice: Standalone Repository Hardening

- Added standalone repository files: `.gitignore`, `pyproject.toml`, and
  `Makefile`.
- Replaced unsupported Factorio `--write-data` usage with a generated local
  `config.ini` passed through `--config`.
- Added generated server settings with `auto_pause=false` so headless runtime
  probes advance ticks with no connected player.
- Added default `open_world` scenario support through
  `qa_control_mod/scenarios/open_world`.
- Made `--factorio-bin` accept either a binary path or an install directory such
  as `/home/user/.factorio`.
- Verified a live local run against `fixtures/mods/qa-broken-mod` using
  `/home/user/.factorio`, which produced missing crafting machine, positive
  output loop, and `script_event_growth` findings plus native saves.
- Replaced FLE-fork documentation in this legacy copy with standalone migration
  notes suitable for publishing `factorio-mod-qa` as its own repository.
- Kept the copied Python modules importable without FLE by using the local
  `prototypes` package.
- Kept the legacy tests as no-Factorio regression coverage while Go ports
  continue.

## Previous Slice: Go Harness Bootstrap

- Bootstrapped `factorio-mod-qa` around a local Factorio subprocess plus RCON
  model.
- Copied selected Python QA modules, prototype helpers, fixtures, and tests into
  `legacy_py/` as temporary porting material.
- Added the Go `fmqa` CLI skeleton, local Factorio process manager, RCON client,
  snapshot validator, report writer, and `qa_control_mod` remote interface.
- Added no-Factorio Go regression tests for snapshot validation, positive-loop
  whitelisting, work-directory creation, staged mod symlinks, RCON response JSON
  extraction, Lua payload quoting, and script-stress entity generation.
