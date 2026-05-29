# Legacy Python QA Modules

This directory is temporary bootstrap code copied from the earlier FLE fork
while the QA behavior is ported to Go.

The copied modules are standalone for offline/static checks:

- prototype helpers import from `legacy_py/prototypes`, not from FLE;
- tests live under `legacy_py/tests`;
- live Factorio process management is owned by the Go `fmqa` CLI;
- container, Compose, FLE cluster, and FLE `FactorioInstance` startup code was
  not copied.

Run the legacy tests from the repository root:

```bash
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=legacy_py python -m pytest legacy_py/tests
```

These modules are not the long-term public API. Treat them as reference
behavior and regression coverage for incremental Go ports.
