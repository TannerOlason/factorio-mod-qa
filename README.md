# factorio-mod-qa

Local-first Factorio mod QA harness. It starts a local Factorio headless binary,
talks to it over RCON, calls the bundled `qa_control_mod`, and writes snapshots,
reports, logs, and native saves under `.fmqa/`.

## Commands

```bash
fmqa run \
  --factorio-bin ~/.factorio \
  --write-dir .fmqa \
  --mods-path /path/to/mods \
  --scenario open_world \
  --qa-scenario all \
  --run-id local-test

fmqa snapshot --rcon localhost:27000 --out prototype_snapshot.json
fmqa validate --snapshot prototype_snapshot.json --reports-dir .fmqa/reports
fmqa validate --config qa_config.json
fmqa blueprint-test ./blueprints/foo.txt
fmqa run --qa-scenario blueprint-smoke --blueprint ./blueprints/foo.txt --blueprint-copies 25 --blueprint-ticks 300
fmqa inspect-mod --mod-dir /path/to/unpacked-mod --mega-base-entities 10000
fmqa doctor --factorio-bin ~/.factorio
```

`fmqa validate` is fully offline. There is no FLE, Docker, or Python runtime path.
During `fmqa run`, user mods and the bundled `qa_control_mod` are symlinked into
`.fmqa/factorio/mods` so Factorio can load one staged mods directory.
`fmqa run` executes short QA scenarios selected with `--qa-scenario`. Current
built-in scenarios are `script-event-growth`, `inventory-abuse`,
`save-load-abuse`, `surface-spawn`, and `blueprint-smoke`; `all` runs every
baseline scenario and includes `blueprint-smoke` when `--blueprint` is set.
`--factorio-bin` may point either to the executable itself or to an install
directory containing `bin/x64/factorio`.
For this machine, `/home/user/.factorio` is the detected install directory.
`fmqa blueprint-test` decodes a Factorio blueprint exchange string or a file
containing either an exchange string or raw blueprint JSON, then prints a JSON
summary, including parametrisation/configuration signals useful for Factorio 2.0
blueprints. `fmqa run --qa-scenario blueprint-smoke --blueprint ...` starts
Factorio, places blueprint ghosts and instant-built entities through
`qa_control_mod`, waits for mod tick reconciliation, and reports placement
failures or configured recipes that persisted after tick logic.
`--blueprint-copies`, `--blueprint-spacing`, and `--blueprint-ticks` turn this
into a small tick-load stress run for blueprint-driven machines.
`fmqa inspect-mod` scans an unpacked mod's Lua source for global tick handlers,
tick fanout, interval-gated storage loops, and rough mega-base loop pressure.

## Runtime Scope

No Docker, Compose, FLE cluster manager, FLE `FactorioInstance`, Docker socket
access, or Python code exists in this project. The default runtime is a normal
local subprocess.

## Verification

Use a writable Go build cache in sandboxed environments:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
GOCACHE=/tmp/go-build-cache go build -buildvcs=false -o /tmp/fmqa ./cmd/fmqa
/tmp/fmqa doctor --factorio-bin /home/user/.factorio
```

Policy config for `fmqa run` and `fmqa validate` is now handled by Go. Supported
config keys include `qa_scenario`, `positive_loop_whitelist`, `suppress_issue_codes`,
`suppress_issue_matches`, `severity_overrides`,
`severity_override_matches`, `static_policy_profiles`,
`enabled_static_policy_profiles`, `min_report_severity`, and `blueprint`.
Blueprint stress config keys include `blueprint_copies`, `blueprint_spacing`,
and `blueprint_ticks`.

Fulgora Prime tick audit used for this slice:

```bash
/tmp/fmqa inspect-mod \
  --mod-dir /home/user/.factorio/mods/fulgora-prime_0.1.0 \
  --mega-base-entities 10000
```

The audit found one global on-tick fanout in `control.lua`, 96 scanned Lua
files, and the main mega-base stress targets documented in
`docs/fulgora-prime-tick-audit.md`.

Blueprint bulk smoke test used for this slice:

```bash
/tmp/fmqa run \
  --factorio-bin /home/user/.factorio \
  --write-dir /tmp/fmqa-blueprint-bulk-live-2 \
  --mods-path fixtures/mods \
  --scenario open_world \
  --qa-scenario blueprint-smoke \
  --blueprint /tmp/fmqa-blueprint-bulk-param-smoke.json \
  --blueprint-copies 3 \
  --blueprint-spacing 12 \
  --blueprint-ticks 60 \
  --run-id blueprint-bulk-smoke-2 \
  --timeout 90s
```

That run placed three instant blueprint copies, waited 60 game ticks, and
reported the three expected configured-recipe persistence observations.

Live smoke test used for this slice:

```bash
/tmp/fmqa run \
  --factorio-bin /home/user/.factorio \
  --write-dir /tmp/fmqa-live \
  --mods-path fixtures/mods \
  --scenario open_world \
  --qa-scenario script-event-growth \
  --run-id scenario-core-smoke \
  --timeout 90s
```

That run produced `recipe_missing_crafting_machine`, `positive_output_loop`, and
`script_event_growth` findings, and wrote native saves under
`/tmp/fmqa-live/factorio/saves`.

Inventory smoke test used for this slice:

```bash
/tmp/fmqa run \
  --factorio-bin /home/user/.factorio \
  --write-dir /tmp/fmqa-inventory-live-3 \
  --mods-path fixtures/mods \
  --scenario open_world \
  --qa-scenario inventory-abuse \
  --run-id inventory-abuse-smoke-3 \
  --timeout 90s
```

That run inserted items into a chest, mined it into a Lua inventory buffer,
returned exactly the expected item counts, wrote a trace artifact, and produced
zero `inventory-abuse` scenario issues.

Save/load smoke test used for this slice:

```bash
/tmp/fmqa run \
  --factorio-bin /home/user/.factorio \
  --write-dir /tmp/fmqa-save-load-live-2 \
  --mods-path fixtures/mods \
  --scenario open_world \
  --qa-scenario save-load-abuse \
  --run-id save-load-smoke-2 \
  --timeout 90s
```

That run saved, restarted Factorio from the native save, verified chest contents
survived reload, mined the chest, and produced zero `save-load-abuse` scenario
issues.

Surface smoke test used for this slice:

```bash
/tmp/fmqa run \
  --factorio-bin /home/user/.factorio \
  --write-dir /tmp/fmqa-surface-live \
  --mods-path fixtures/mods \
  --scenario open_world \
  --qa-scenario surface-spawn \
  --run-id surface-spawn-smoke \
  --timeout 90s
```

That run created a temporary surface, found a buildable position near origin,
placed and mined a chest, deleted the temporary surface, and produced zero
`surface-spawn` scenario issues.

Blueprint foundation smoke test used for this slice:

```bash
/tmp/fmqa blueprint-test /tmp/fmqa-blueprint-smoke.json
```

That run decoded raw blueprint JSON and printed a summary with blueprint kind,
label, entity count, and tile count.

Blueprint in-game smoke support added in this slice:

```bash
/tmp/fmqa run \
  --factorio-bin /home/user/.factorio \
  --write-dir /tmp/fmqa-blueprint-live \
  --mods-path fixtures/mods \
  --scenario open_world \
  --qa-scenario blueprint-smoke \
  --blueprint /tmp/fmqa-blueprint-smoke.json \
  --run-id blueprint-smoke \
  --timeout 90s
```

The scenario preserves raw blueprint JSON, including 2.0 parametrisation fields,
then exercises both ghost placement and instant construction in Factorio.
