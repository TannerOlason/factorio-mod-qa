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
fmqa doctor --factorio-bin ~/.factorio
```

`fmqa validate` is fully offline. There is no FLE, Docker, or Python runtime path.
During `fmqa run`, user mods and the bundled `qa_control_mod` are symlinked into
`.fmqa/factorio/mods` so Factorio can load one staged mods directory.
`fmqa run` executes short QA scenarios selected with `--qa-scenario`. Current
built-in scenarios are `script-event-growth`, `inventory-abuse`, and
`save-load-abuse`; `all` runs every built-in scenario.
`--factorio-bin` may point either to the executable itself or to an install
directory containing `bin/x64/factorio`.
For this machine, `/home/user/.factorio` is the detected install directory.

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
`enabled_static_policy_profiles`, and `min_report_severity`.

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
