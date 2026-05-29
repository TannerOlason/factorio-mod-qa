from __future__ import annotations

import argparse
import datetime as dt
import json
from pathlib import Path
from typing import Any

from mod_qa_agent.fuzz_orchestrator import FuzzOrchestrator
from mod_qa_agent.snapshot_session import SnapshotSession

DEFAULT_OPTIONS: dict[str, Any] = {
    "extends": [],
    "rcon_port": 27000,
    "address": "localhost",
    "run_id": "auto",
    "output_dir": ".fle/mod_qa",
    "snapshot": None,
    "seed": None,
    "goals": 20,
    "max_traces": None,
    "llm_planning": False,
    "native_saves": False,
    "mutations": 0,
    "positive_loop_whitelist": [],
    "suppress_issue_codes": [],
    "suppress_issue_matches": [],
    "severity_overrides": {},
    "severity_override_matches": [],
    "static_policy_profiles": {},
    "enabled_static_policy_profiles": [],
    "min_report_severity": "info",
    "validate_only": False,
    "start_cluster": False,
    "mods_path": None,
    "scenario": "open_world",
}

LIST_CONFIG_KEYS = {
    "positive_loop_whitelist",
    "suppress_issue_codes",
    "suppress_issue_matches",
    "severity_override_matches",
    "enabled_static_policy_profiles",
}

SCALAR_LIST_CONFIG_KEYS = {
    "positive_loop_whitelist",
    "suppress_issue_codes",
    "enabled_static_policy_profiles",
}

DICT_CONFIG_KEYS = {"severity_overrides", "static_policy_profiles"}
STATIC_POLICY_PROFILE_KEYS = {
    "positive_loop_whitelist",
    "suppress_issue_codes",
    "suppress_issue_matches",
    "severity_overrides",
    "severity_override_matches",
    "min_report_severity",
}


def _run_id(value: str) -> str:
    if value == "auto":
        return dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return value


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Run deterministic Factorio mod QA checks.")
    parser.add_argument("--config", help="Path to a JSON runner config")
    parser.add_argument("--rcon-port", type=int)
    parser.add_argument("--address")
    parser.add_argument("--run-id")
    parser.add_argument("--output-dir")
    parser.add_argument(
        "--snapshot",
        help="Path to an existing prototype_snapshot.json for offline validation",
    )
    parser.add_argument("--seed", type=int)
    parser.add_argument("--goals", type=int)
    parser.add_argument("--max-traces", type=int)
    parser.add_argument("--llm-planning", action="store_true", default=None)
    parser.add_argument(
        "--native-saves",
        action="store_true",
        default=None,
        help="Write native Factorio .zip saves for archived states when possible",
    )
    parser.add_argument(
        "--mutations",
        type=int,
        help="Number of mutated traces to append after deterministic seed traces",
    )
    parser.add_argument(
        "--positive-loop-whitelist",
        action="append",
        help="Recipe name to ignore for positive same-material output loop checks",
    )
    parser.add_argument(
        "--suppress-issue-code",
        action="append",
        dest="suppress_issue_codes",
        help="Static validator issue code to suppress",
    )
    parser.add_argument(
        "--min-report-severity",
        choices=["info", "warning", "error"],
        help="Minimum static issue severity that should produce markdown reports",
    )
    parser.add_argument(
        "--validate-only",
        action="store_true",
        default=None,
        help="Export prototypes and run static validators without executing traces",
    )
    parser.add_argument("--start-cluster", action="store_true", default=None)
    parser.add_argument("--mods-path")
    parser.add_argument("--scenario")
    return parser


def _merge_config(base: dict[str, Any], override: dict[str, Any]) -> dict[str, Any]:
    merged = dict(base)
    for key, value in override.items():
        if key == "extends":
            continue
        if key in LIST_CONFIG_KEYS:
            merged[key] = list(merged.get(key) or []) + _config_list(key, value)
        elif key in DICT_CONFIG_KEYS:
            merged[key] = {**dict(merged.get(key) or {}), **_config_dict(key, value)}
        else:
            merged[key] = value
    return merged


def _config_list(key: str, value: Any) -> list[Any]:
    if value is None:
        return []
    if isinstance(value, list):
        return list(value)
    if isinstance(value, str) and key in SCALAR_LIST_CONFIG_KEYS:
        return [value]
    raise ValueError(f"Runner config option {key} must be a JSON array")


def _config_dict(key: str, value: Any) -> dict[str, Any]:
    if value is None:
        return {}
    if not isinstance(value, dict):
        raise ValueError(f"Runner config option {key} must be a JSON object")
    return dict(value)


def _apply_static_policy_profiles(config: dict[str, Any]) -> dict[str, Any]:
    enabled = config.get("enabled_static_policy_profiles") or []
    if isinstance(enabled, str):
        enabled = [enabled]
    if not isinstance(enabled, list):
        raise ValueError("enabled_static_policy_profiles must be a string or list")

    profiles = config.get("static_policy_profiles") or {}
    if not isinstance(profiles, dict):
        raise ValueError("static_policy_profiles must be a JSON object")

    profile_options: dict[str, Any] = {}
    for name in enabled:
        profile = profiles.get(name)
        if profile is None:
            raise ValueError(f"Unknown static policy profile: {name}")
        if not isinstance(profile, dict):
            raise ValueError(f"Static policy profile {name} must be a JSON object")
        unknown = sorted(set(profile) - STATIC_POLICY_PROFILE_KEYS)
        if unknown:
            raise ValueError(
                f"Unknown static policy profile option(s) in {name}: "
                + ", ".join(unknown)
            )
        profile_options = _merge_config(profile_options, profile)

    return _merge_config(profile_options, config)


def _load_config(path: str | None, seen: set[Path] | None = None) -> dict[str, Any]:
    if not path:
        return {}
    config_path = Path(path).expanduser()
    if not config_path.is_absolute():
        config_path = config_path.resolve()
    seen = seen or set()
    if config_path in seen:
        raise ValueError(f"Config extends cycle detected at {config_path}")
    seen.add(config_path)

    with config_path.open("r", encoding="utf-8") as f:
        data = json.load(f)
    if not isinstance(data, dict):
        raise ValueError("Runner config must be a JSON object")
    unknown = sorted(set(data) - set(DEFAULT_OPTIONS))
    if unknown:
        raise ValueError(f"Unknown runner config option(s): {', '.join(unknown)}")

    extends = data.get("extends") or []
    if isinstance(extends, str):
        extends = [extends]
    if not isinstance(extends, list):
        raise ValueError("Runner config extends must be a string or list")

    inherited: dict[str, Any] = {}
    for parent in extends:
        parent_path = Path(parent).expanduser()
        if not parent_path.is_absolute():
            parent_path = config_path.parent / parent_path
        inherited = _merge_config(
            inherited,
            _load_config(str(parent_path), seen),
        )
    merged = _merge_config(inherited, data)
    seen.remove(config_path)
    return merged


def _merge_list_option(
    options: dict[str, Any], parsed: argparse.Namespace, key: str
) -> None:
    value = getattr(parsed, key, None)
    if value is not None:
        existing = options.get(key) or []
        options[key] = list(existing) + list(value)


def resolve_options(argv: list[str] | None = None) -> argparse.Namespace:
    parsed = build_parser().parse_args(argv)
    options = dict(DEFAULT_OPTIONS)
    options.update(_apply_static_policy_profiles(_load_config(parsed.config)))

    list_keys = {"positive_loop_whitelist", "suppress_issue_codes"}
    for key in DEFAULT_OPTIONS:
        if key in list_keys:
            continue
        value = getattr(parsed, key, None)
        if value is not None:
            options[key] = value

    for key in list_keys:
        _merge_list_option(options, parsed, key)

    if parsed.llm_planning:
        options["llm_planning"] = True
    if parsed.native_saves:
        options["native_saves"] = True
    if parsed.start_cluster:
        options["start_cluster"] = True
    if parsed.validate_only:
        options["validate_only"] = True

    if options["min_report_severity"] not in {"info", "warning", "error"}:
        raise ValueError("min_report_severity must be one of: info, warning, error")
    if options["snapshot"] and options["start_cluster"]:
        raise ValueError("--snapshot cannot be combined with --start-cluster")
    if options["snapshot"]:
        options["validate_only"] = True

    options["config"] = parsed.config
    return argparse.Namespace(**options)


def main(argv: list[str] | None = None) -> int:
    args = resolve_options(argv)
    output_dir = Path(args.output_dir)
    run_id = _run_id(args.run_id)
    run_dir = output_dir / "runs" / run_id
    reports_dir = output_dir / "reports"

    if args.snapshot:
        session = SnapshotSession(args.snapshot)
    elif args.start_cluster:
        from mod_qa_agent.factorio_session import FactorioSession

        session = FactorioSession.start_cluster(
            scenario=args.scenario,
            mods_path=args.mods_path,
            rcon_port=args.rcon_port,
        )
    else:
        from mod_qa_agent.factorio_session import FactorioSession

        session = FactorioSession(
            address=args.address,
            rcon_port=args.rcon_port,
            all_technologies_researched=False,
        )

    try:
        summary = FuzzOrchestrator(
            session=session,
            run_dir=run_dir,
            reports_dir=reports_dir,
            seed=args.seed,
            goals=args.goals,
            max_traces=args.max_traces,
            llm_planning=args.llm_planning,
            native_saves=args.native_saves,
            mods_path=args.mods_path,
            mutations=args.mutations,
            positive_loop_whitelist=args.positive_loop_whitelist,
            suppress_issue_codes=args.suppress_issue_codes,
            suppress_issue_matches=args.suppress_issue_matches,
            severity_overrides=args.severity_overrides,
            severity_override_matches=args.severity_override_matches,
            min_report_severity=args.min_report_severity,
            validate_only=args.validate_only,
        ).run()
    finally:
        session.close()

    print(f"Run complete: {summary['run_dir']}")
    print(f"Reports written: {len(summary['reports'])}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
