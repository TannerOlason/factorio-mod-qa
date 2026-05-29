# Architecture

`factorio-mod-qa` is a local-first Factorio mod QA harness. The default path is
one Go CLI process plus one local Factorio headless process.

```text
cmd/fmqa
  internal/factorio   starts/stops Factorio and prepares .fmqa directories
  internal/qa         runs short QA scenarios and writes action traces
  internal/rcon       speaks Source RCON over TCP
  internal/runner     orchestrates run/snapshot/validate flows
  internal/snapshot   loads snapshots and runs static graph validation
  internal/reports    writes JSON and Markdown summaries
  qa_control_mod      Factorio remote interface used by RCON commands
```

The runtime deliberately has no Docker or Compose integration. A future CI path
can add containers as an optional wrapper, but the developer workflow stays
local Factorio plus RCON.

## CLI Commands

`fmqa run` starts Factorio, waits for RCON, exports a prototype snapshot through
`qa_control_mod`, runs static validation, runs selected short QA scenarios,
writes reports and scenario traces, and requests a native Factorio save.
Built-in QA scenarios currently include `script-event-growth`, `inventory-abuse`,
and `save-load-abuse`.

Before startup, `fmqa run` stages a single Factorio mods directory at
`.fmqa/factorio/mods` by symlinking entries from `--mods-path` and the bundled
`qa_control_mod`. This keeps the control API available while preserving the
user's original mods directory.

The launcher writes a local Factorio `config.ini` under `.fmqa/factorio/config`
and points Factorio at it with `--config`. The config keeps Factorio's
`write-data` under `.fmqa/factorio` while deriving `read-data` from the selected
Factorio binary, so the harness does not mutate the user's normal Factorio
profile.

The launcher also writes local server settings with `auto_pause` disabled.
Runtime probes depend on ticks advancing even when no player is connected.

`fmqa snapshot` connects to an already-running Factorio RCON server and writes
`prototype_snapshot.json`.

`fmqa validate` validates an existing snapshot without starting Factorio.

`fmqa run` and `fmqa validate` both accept JSON policy config through
`--config`. Static policy handling is implemented in Go and supports extended
configs, reusable static policy profiles, suppressions, severity overrides, and
minimum report severity.

`fmqa doctor` checks that the selected Factorio install resolves, can report a
version, and that `qa_control_mod/info.json` is present.

## QA Control Mod

The bundled mod exposes these remote calls:

- `export_snapshot`
- `runtime_summary`
- `place_entities_batch`
- `place_entity`
- `snapshot_state`
- `insert_items`
- `read_entity_inventory`
- `mine_entity_to_inventory`
- `remove_entity`
- `advance_ticks`
- `script_event_counts`
- `reset_script_event_counts`
- `save`
- `dispatch`

Snapshot export serializes all available prototype sections, including Space Age
prototype data when it is loaded by the local Factorio install. The harness does
not disable DLC or strip mod data by default.

Scenario calls use the `dispatch` remote interface. Dispatch accepts a command
name plus JSON payload and returns a JSON envelope, so Go scenarios can record
deterministic action/observation traces without parsing ad hoc Lua responses.
Scenarios can also request a save/restart cycle; the runner stops Factorio,
starts it again from the named native save, reconnects RCON, and keeps the same
scenario trace open across the reload.
