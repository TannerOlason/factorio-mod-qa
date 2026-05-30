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
`save-load-abuse`, `surface-spawn`, and `blueprint-smoke`.

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

`fmqa blueprint-test` decodes a Factorio blueprint exchange string or a file
containing an exchange string/raw blueprint JSON and prints a structured summary.
The summary preserves raw JSON and highlights parametrisation/configuration
fields such as recipes, control behavior, item requests, and inventory settings.

`fmqa run --qa-scenario blueprint-smoke --blueprint ...` sends the decoded JSON
to `qa_control_mod`, places ghosts, instant-builds real entities with build
events raised, waits for tick-driven mod reconciliation, and reports placement
failures or blueprint-configured recipes that remain applied. This is intended
to catch mod-agnostic versions of player escape paths where a blueprint carries
recipe, signal, module, equipment, or inventory state into a script-controlled
machine. `--blueprint-copies`, `--blueprint-spacing`, and `--blueprint-ticks`
scale the same scenario into a repeat placement/tick-load probe.

`fmqa inspect-mod` scans an unpacked mod's Lua source for on-tick handlers, tick
fanout calls, interval constants, storage-backed loops inside tick contexts, and
rough mega-base loop pressure estimates. This is source-level guidance for which
runtime scenarios to build; it does not replace live Factorio timing.

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
- `create_surface`
- `delete_surface`
- `find_buildable_position`
- `insert_items`
- `read_entity_inventory`
- `mine_entity_to_inventory`
- `remove_entity`
- `blueprint_smoke`
- `read_tracked_entities`
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
