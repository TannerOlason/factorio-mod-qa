# Development Checklist

## Done

- [x] Local Go CLI with no Docker, FLE, or Python runtime.
- [x] Local Factorio process launch, staged mods directory, local write-data, and RCON connection.
- [x] `qa_control_mod` prototype snapshot export and JSON command dispatcher.
- [x] Offline static prototype validation and Go static policy config.
- [x] Run-level JSON/Markdown reports with artifacts and per-scenario traces.
- [x] Native save artifact creation.
- [x] Scenario core with deterministic setup/run/check/cleanup lifecycle.
- [x] `script-event-growth` runtime scenario.
- [x] `inventory-abuse` runtime scenario.
- [x] `save-load-abuse` runtime scenario with Factorio restart from native save.
- [x] `surface-spawn` runtime scenario with temporary surface lifecycle.
- [x] Removed `legacy_py/` and `pyproject.toml`.
- [x] Blueprint foundation: decode Factorio blueprint exchange strings/raw JSON in Go and expose `fmqa blueprint-test`.
- [x] Blueprint feature inspection for parametrisation/configured recipes/control behavior/item requests.

## Done In This Slice

- [x] Blueprint in-game smoke scenario: send decoded blueprint JSON through `qa_control_mod`, validate ghost/instant placement outcomes, and report missing entities or configured recipes that persisted after tick reconciliation.
- [x] Added Factorio 2.0 parametrisation-aware fixture coverage in Go blueprint decoding.
- [x] Took note of `fulgora-prime` puzzle machines and inhibition cubes as target patterns: script-owned recipe reconciliation from circuit signals and script-owned module-slot insertion.
- [x] Added `fmqa inspect-mod` source audit for tick handlers, tick fanout, storage-backed loops, and mega-base loop-pressure estimates.
- [x] Audited `fulgora-prime` tick-script targets and documented the result in `docs/fulgora-prime-tick-audit.md`.
- [x] Extended `blueprint-smoke` into a repeat placement/tick-load probe with `--blueprint-copies`, `--blueprint-spacing`, and `--blueprint-ticks`.

## Next

- [ ] Fulgora Prime focused blueprint fixtures: parametrized blueprints carrying puzzle-machine recipes, control behavior, item requests, and module/equipment slot payloads.
- [ ] Lua/runtime timing polish: collect Factorio-side elapsed tick timing or UPS-sensitive script timing for repeat blueprint runs.
- [ ] Planet/surface-specific scenario: enumerate or select modded surfaces/planets, verify spawn reachability and starter-resource presence.
- [ ] Broaden inventory abuse: place/remove loops, module/productivity/recycling probes, and before/after item conservation checks.
- [ ] Static validator improvements: unreachable recipes/tech chains, unintended module eligibility, and recipe-category coverage.
- [ ] Report polish: per-issue repro steps and compact scenario-focused summaries.
