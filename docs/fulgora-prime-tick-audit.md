# Fulgora Prime Tick Audit

Source inspected: `/home/user/.factorio/mods/fulgora-prime_0.1.0`, resolved to
`/home/user/Documents/Projects/fulgora-prime`.

Command:

```bash
/tmp/fmqa inspect-mod \
  --mod-dir /home/user/.factorio/mods/fulgora-prime_0.1.0 \
  --mega-base-entities 10000
```

## Summary

- Scanned 96 Lua files, excluding tests and vendored dependencies.
- Found one global `script.on_event(defines.events.on_tick, ...)` handler in
  `control.lua`.
- The global handler fans out to tick modules including puzzle machines,
  stock market/terminals, gambling buildings, mystery terminals, orbital ships,
  convoy, quadrail, system2, system3, and system4.
- At an assumed 10,000 entries in each entity-backed storage table, the static
  heuristic reported roughly 6.1M storage-loop iterations per second across
  tick-context loops.

## Important Targets

- `scripts/phases/phase-01-nauvis-sidegrades/puzzle-machine.lua`
  scans `storage.fp_puzzle_machines` every `SYNC_INTERVAL=60`.
  At 10,000 puzzle machines, that is 10,000 iterations/sec on average and a
  10,000-entry burst once per second. This is the primary recipe-reconciliation
  blueprint target.
- `scripts/phases/phase-02-fulgora-prime/gambling_buildings.lua`
  scans `storage.fp_gambling_machines` without a detected interval guard.
  At 10,000 machines, that models as 600,000 iterations/sec.
- `scripts/phases/phase-02-fulgora-prime/mysterious_terminal.lua`
  scans `storage.fp_terminals` in terminal scan code and refresh state.
  Terminals are a relevant blueprint/config target because they interact with
  circuit/stock state.
- `scripts/phases/phase-02-fulgora-prime/stock_market.lua`
  scans `storage.fp_terminals` during stock tick handling. Some paths are
  interval-gated by day or price intervals, but terminal scans still need
  runtime verification.
- `scripts/phases/phase-01-nauvis-sidegrades/knotspace.lua`,
  `dispersal_node.lua`, `gevulot.lua`, `gestalt.lua`, and `cryo_modules.lua`
  have entity-backed periodic scans that should be included in larger
  blueprint/placement stress suites.

## Mega-Base Interpretation

The scanner estimates average loop pressure with:

```text
iterations_per_second = table_entries * 60 / interval_ticks
```

This is intentionally conservative. A loop gated every 60 ticks over 10,000
entities averages 10,000 iterations/sec, but it still performs a 10,000-entry
burst on the sweep tick. A loop without an interval guard over 10,000 entities
does 10,000 iterations every tick, or 600,000 iterations/sec.

At 60 UPS the whole simulation has about 16.67 ms/tick. A 1 ms/tick script
budget supports only about 1,000 iterations/tick if each iteration costs 1 us.
Many Factorio Lua iterations are more expensive than that when they touch
inventories, surfaces, control behaviors, circuit networks, or entity state, so
the practical limit can be much lower.

## Next Runtime Stress Targets

- Generate parametrized blueprints for `fp-lights-out-machine`,
  `fp-minesweeper-machine`, `fp-obstacle-nav-machine`, and
  `fp-threshold-tuning-assembler` with recipes/control behavior/item requests.
- Use `blueprint-smoke` with `--blueprint-copies`, `--blueprint-spacing`, and
  `--blueprint-ticks` to place N copies of a decoded blueprint and measure
  tick/reconciliation outcomes.
- Add a tick-load scenario that can place representative entities from detected
  storage loops and run a fixed tick window while collecting Factorio elapsed
  time and scenario traces.
