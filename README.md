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
  --run-id local-test

fmqa snapshot --rcon localhost:27000 --out prototype_snapshot.json
fmqa validate --snapshot prototype_snapshot.json --reports-dir .fmqa/reports
fmqa validate --config qa_config.json
fmqa doctor --factorio-bin ~/.factorio
```

`fmqa validate` is fully offline and does not import FLE. The copied Python code
under `legacy_py/` is reference material for behavior still being ported to Go;
the default build and test path does not execute it.
During `fmqa run`, user mods and the bundled `qa_control_mod` are symlinked into
`.fmqa/factorio/mods` so Factorio can load one staged mods directory.
If the live snapshot contains `qa-ticking-machine`, the runner places a small
batch and reports `script_event_growth` when the fixture counter grows.
`--factorio-bin` may point either to the executable itself or to an install
directory containing `bin/x64/factorio`.
For this machine, `/home/user/.factorio` is the detected install directory.

## Runtime Scope

No Docker, Compose, FLE cluster manager, FLE `FactorioInstance`, or Docker socket
access exists in this project. The default runtime is a normal local subprocess.
The historical Python code under `legacy_py/` is self-contained for offline
tests and does not import FLE.

## Verification

Use a writable Go build cache in sandboxed environments:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
GOCACHE=/tmp/go-build-cache go build -buildvcs=false -o /tmp/fmqa ./cmd/fmqa
/tmp/fmqa doctor --factorio-bin /home/user/.factorio
```

Policy config for `fmqa run` and `fmqa validate` is now handled by Go. Supported
static policy keys include `positive_loop_whitelist`, `suppress_issue_codes`,
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
  --run-id live-smoke \
  --timeout 90s
```

That run produced `recipe_missing_crafting_machine`, `positive_output_loop`, and
`script_event_growth` findings, and wrote native saves under
`/tmp/fmqa-live/factorio/saves`.
