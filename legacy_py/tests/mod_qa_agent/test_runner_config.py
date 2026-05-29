import json

import pytest

from mod_qa_agent.runner import resolve_options

pytestmark = pytest.mark.no_factorio


def test_resolve_options_loads_json_config(tmp_path):
    config_path = tmp_path / "qa_config.json"
    config_path.write_text(
        json.dumps(
            {
                "goals": 7,
                "max_traces": 3,
                "mutations": 2,
                "output_dir": ".fle/custom_qa",
                "snapshot": "prototype_snapshot.json",
                "native_saves": True,
                "suppress_issue_codes": ["item_without_use"],
                "suppress_issue_matches": [
                    {
                        "code": "recipe_missing_crafting_machine",
                        "details": {"recipe": "debug-plate"},
                    }
                ],
                "severity_overrides": {"positive_output_loop": "info"},
                "severity_override_matches": [
                    {
                        "code": "fluid_source_sink_gap",
                        "details": {"fluid": "debug-fluid"},
                        "severity": "info",
                    }
                ],
                "min_report_severity": "warning",
                "validate_only": True,
            }
        ),
        encoding="utf-8",
    )

    options = resolve_options(["--config", str(config_path)])

    assert options.goals == 7
    assert options.max_traces == 3
    assert options.mutations == 2
    assert options.output_dir == ".fle/custom_qa"
    assert options.snapshot == "prototype_snapshot.json"
    assert options.native_saves is True
    assert options.suppress_issue_codes == ["item_without_use"]
    assert options.suppress_issue_matches == [
        {
            "code": "recipe_missing_crafting_machine",
            "details": {"recipe": "debug-plate"},
        }
    ]
    assert options.severity_overrides == {"positive_output_loop": "info"}
    assert options.severity_override_matches == [
        {
            "code": "fluid_source_sink_gap",
            "details": {"fluid": "debug-fluid"},
            "severity": "info",
        }
    ]
    assert options.min_report_severity == "warning"
    assert options.validate_only is True


def test_resolve_options_cli_overrides_config(tmp_path):
    config_path = tmp_path / "qa_config.json"
    config_path.write_text(
        json.dumps({"goals": 7, "mutations": 2}),
        encoding="utf-8",
    )

    options = resolve_options(
        ["--config", str(config_path), "--goals", "4", "--mutations", "1"]
    )

    assert options.goals == 4
    assert options.mutations == 1


def test_resolve_options_loads_extended_config_profiles(tmp_path):
    profile_path = tmp_path / "shared_policy.json"
    profile_path.write_text(
        json.dumps(
            {
                "goals": 9,
                "suppress_issue_codes": ["item_without_use"],
                "severity_overrides": {"fluid_source_sink_gap": "info"},
            }
        ),
        encoding="utf-8",
    )
    config_path = tmp_path / "modpack.json"
    config_path.write_text(
        json.dumps(
            {
                "extends": "shared_policy.json",
                "goals": 3,
                "suppress_issue_codes": ["fluid_source_sink_gap"],
                "severity_overrides": {"positive_output_loop": "warning"},
            }
        ),
        encoding="utf-8",
    )

    options = resolve_options(["--config", str(config_path)])

    assert options.goals == 3
    assert options.suppress_issue_codes == [
        "item_without_use",
        "fluid_source_sink_gap",
    ]
    assert options.severity_overrides == {
        "fluid_source_sink_gap": "info",
        "positive_output_loop": "warning",
    }


def test_resolve_options_applies_static_policy_profiles(tmp_path):
    config_path = tmp_path / "qa_config.json"
    config_path.write_text(
        json.dumps(
            {
                "enabled_static_policy_profiles": ["quiet-base"],
                "static_policy_profiles": {
                    "quiet-base": {
                        "suppress_issue_codes": ["item_without_use"],
                        "severity_overrides": {"fluid_source_sink_gap": "info"},
                    }
                },
                "suppress_issue_codes": ["positive_output_loop"],
                "severity_overrides": {"item_without_use": "warning"},
            }
        ),
        encoding="utf-8",
    )

    options = resolve_options(["--config", str(config_path)])

    assert options.suppress_issue_codes == [
        "item_without_use",
        "positive_output_loop",
    ]
    assert options.severity_overrides == {
        "fluid_source_sink_gap": "info",
        "item_without_use": "warning",
    }


def test_resolve_options_accepts_scalar_string_list_config_fields(tmp_path):
    config_path = tmp_path / "qa_config.json"
    config_path.write_text(
        json.dumps(
            {
                "enabled_static_policy_profiles": "quiet-base",
                "static_policy_profiles": {
                    "quiet-base": {
                        "suppress_issue_codes": "item_without_use",
                    }
                },
                "positive_loop_whitelist": "known-loop",
            }
        ),
        encoding="utf-8",
    )

    options = resolve_options(["--config", str(config_path)])

    assert options.enabled_static_policy_profiles == ["quiet-base"]
    assert options.suppress_issue_codes == ["item_without_use"]
    assert options.positive_loop_whitelist == ["known-loop"]


def test_resolve_options_applies_inherited_static_policy_profiles_once(tmp_path):
    profile_path = tmp_path / "shared_policy.json"
    profile_path.write_text(
        json.dumps(
            {
                "enabled_static_policy_profiles": ["quiet-base"],
                "static_policy_profiles": {
                    "quiet-base": {
                        "suppress_issue_codes": ["item_without_use"],
                    }
                },
            }
        ),
        encoding="utf-8",
    )
    config_path = tmp_path / "modpack.json"
    config_path.write_text(
        json.dumps(
            {
                "extends": "shared_policy.json",
                "suppress_issue_codes": ["positive_output_loop"],
            }
        ),
        encoding="utf-8",
    )

    options = resolve_options(["--config", str(config_path)])

    assert options.suppress_issue_codes == [
        "item_without_use",
        "positive_output_loop",
    ]


def test_resolve_options_rejects_unknown_static_policy_profile(tmp_path):
    config_path = tmp_path / "qa_config.json"
    config_path.write_text(
        json.dumps({"enabled_static_policy_profiles": ["missing"]}),
        encoding="utf-8",
    )

    with pytest.raises(ValueError, match="Unknown static policy profile"):
        resolve_options(["--config", str(config_path)])


def test_resolve_options_appends_list_flags_to_config(tmp_path):
    config_path = tmp_path / "qa_config.json"
    config_path.write_text(
        json.dumps(
            {
                "positive_loop_whitelist": ["kovarex-enrichment-process"],
                "suppress_issue_codes": ["item_without_use"],
            }
        ),
        encoding="utf-8",
    )

    options = resolve_options(
        [
            "--config",
            str(config_path),
            "--positive-loop-whitelist",
            "modded-recycling",
            "--suppress-issue-code",
            "fluid_source_sink_gap",
        ]
    )

    assert options.positive_loop_whitelist == [
        "kovarex-enrichment-process",
        "modded-recycling",
    ]
    assert options.suppress_issue_codes == [
        "item_without_use",
        "fluid_source_sink_gap",
    ]


def test_resolve_options_rejects_unknown_config_keys(tmp_path):
    config_path = tmp_path / "qa_config.json"
    config_path.write_text(json.dumps({"surprise": True}), encoding="utf-8")

    with pytest.raises(ValueError, match="Unknown runner config"):
        resolve_options(["--config", str(config_path)])


def test_resolve_options_rejects_invalid_structured_list_config_field(tmp_path):
    config_path = tmp_path / "qa_config.json"
    config_path.write_text(
        json.dumps({"suppress_issue_matches": "recipe_missing_crafting_machine"}),
        encoding="utf-8",
    )

    with pytest.raises(ValueError, match="suppress_issue_matches"):
        resolve_options(["--config", str(config_path)])


def test_resolve_options_rejects_invalid_dict_config_field(tmp_path):
    config_path = tmp_path / "qa_config.json"
    config_path.write_text(
        json.dumps({"severity_overrides": ["positive_output_loop"]}),
        encoding="utf-8",
    )

    with pytest.raises(ValueError, match="severity_overrides"):
        resolve_options(["--config", str(config_path)])


def test_resolve_options_rejects_bad_min_report_severity(tmp_path):
    config_path = tmp_path / "qa_config.json"
    config_path.write_text(
        json.dumps({"min_report_severity": "very-serious"}),
        encoding="utf-8",
    )

    with pytest.raises(ValueError, match="min_report_severity"):
        resolve_options(["--config", str(config_path)])


def test_resolve_options_snapshot_implies_validate_only():
    options = resolve_options(["--snapshot", "prototype_snapshot.json"])

    assert options.snapshot == "prototype_snapshot.json"
    assert options.validate_only is True


def test_resolve_options_snapshot_rejects_start_cluster():
    with pytest.raises(ValueError, match="snapshot"):
        resolve_options(["--snapshot", "prototype_snapshot.json", "--start-cluster"])
