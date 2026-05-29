import pytest

from mod_qa_agent.static_analyzer import StaticAnalyzer, StaticIssuePolicy

pytestmark = pytest.mark.no_factorio


class FakeSession:
    def export_prototype_snapshot(self):
        return {
            "schema_version": 1,
            "factorio_version": "2.0.73",
            "active_mods": {"base": "2.0.73"},
            "items": {"iron-ore": {}, "iron-plate": {}},
            "fluids": {},
            "resources": {
                "iron-ore": {
                    "mineable_properties": {
                        "products": [{"name": "iron-ore", "type": "item", "amount": 1}]
                    }
                }
            },
            "recipes": {
                "iron-plate": {
                    "name": "iron-plate",
                    "category": "missing-smelting",
                    "ingredients": [{"name": "iron-ore", "type": "item", "amount": 1}],
                    "products": [{"name": "iron-plate", "type": "item", "amount": 1}],
                }
            },
            "entities": {},
            "technologies": {},
        }


def test_static_analyzer_applies_issue_policy():
    policy = StaticIssuePolicy.from_options(
        suppress_issue_codes=["recipe_missing_crafting_machine"],
        severity_overrides={"item_without_use": "warning"},
        min_report_severity="warning",
    )

    result = StaticAnalyzer(policy).analyze(FakeSession().export_prototype_snapshot())

    assert result.summary_counts() == {
        "raw_static_issue_count": 2,
        "static_issue_count": 1,
        "reportable_static_issue_count": 1,
        "suppressed_static_issue_count": 1,
    }
    assert result.issues[0].code == "item_without_use"
    assert result.issues[0].severity == "warning"
    assert result.reportable_issues == result.issues


def test_static_analyzer_can_suppress_via_severity_override():
    policy = StaticIssuePolicy.from_options(
        severity_overrides={
            "recipe_missing_crafting_machine": "ignore",
            "item_without_use": "suppress",
        }
    )

    result = StaticAnalyzer(policy).analyze(FakeSession().export_prototype_snapshot())

    assert result.raw_issues
    assert result.issues == []
    assert result.reportable_issues == []


def test_static_analyzer_can_suppress_exact_issue_matches():
    policy = StaticIssuePolicy.from_options(
        suppress_issue_matches=[
            {
                "code": "recipe_missing_crafting_machine",
                "details": {"recipe": "iron-plate", "category": "missing-smelting"},
            }
        ],
    )

    result = StaticAnalyzer(policy).analyze(FakeSession().export_prototype_snapshot())

    assert result.summary_counts() == {
        "raw_static_issue_count": 2,
        "static_issue_count": 1,
        "reportable_static_issue_count": 1,
        "suppressed_static_issue_count": 1,
    }
    assert [issue.code for issue in result.issues] == ["item_without_use"]


def test_static_analyzer_can_override_exact_issue_matches():
    policy = StaticIssuePolicy.from_options(
        severity_override_matches=[
            {
                "code": "recipe_missing_crafting_machine",
                "details": {"recipe": "iron-plate"},
                "severity": "info",
            }
        ],
        min_report_severity="warning",
    )

    result = StaticAnalyzer(policy).analyze(FakeSession().export_prototype_snapshot())

    issue_by_code = {issue.code: issue for issue in result.issues}
    assert issue_by_code["recipe_missing_crafting_machine"].severity == "info"
    assert result.reportable_issues == []


def test_static_analyzer_can_suppress_contained_issue_details():
    snapshot = FakeSession().export_prototype_snapshot()
    snapshot["recipes"]["broken-widget"] = {
        "name": "broken-widget",
        "category": "missing-widget-crafting",
        "ingredients": [
            {"name": "missing-widget-part", "type": "item", "amount": 1},
        ],
        "products": [{"name": "iron-plate", "type": "item", "amount": 1}],
    }
    policy = StaticIssuePolicy.from_options(
        suppress_issue_matches=[
            {
                "code": "recipe_missing_ingredient_prototype",
                "details_contains": {"ingredients": "missing-widget-part"},
            },
            {
                "code": "recipe_missing_crafting_machine",
                "details_contains": {"category": "missing-widget"},
            },
        ],
    )

    result = StaticAnalyzer(policy).analyze(snapshot)

    codes = {issue.code for issue in result.issues}
    assert "recipe_missing_ingredient_prototype" not in codes
    assert all(
        issue.details.get("recipe") != "broken-widget"
        for issue in result.issues
        if issue.code == "recipe_missing_crafting_machine"
    )


def test_static_analyzer_can_override_contained_issue_details():
    snapshot = FakeSession().export_prototype_snapshot()
    snapshot["recipes"]["positive-loop"] = {
        "name": "positive-loop",
        "category": "crafting",
        "ingredients": [{"name": "iron-plate", "type": "item", "amount": 1}],
        "products": [{"name": "iron-plate", "type": "item", "amount": 2}],
    }
    policy = StaticIssuePolicy.from_options(
        severity_override_matches=[
            {
                "code": "positive_output_loop",
                "details_contains": {"materials": "iron-plate"},
                "severity": "info",
            }
        ],
        min_report_severity="warning",
    )

    result = StaticAnalyzer(policy).analyze(snapshot)

    issue_by_code = {issue.code: issue for issue in result.issues}
    assert issue_by_code["positive_output_loop"].severity == "info"
    assert "positive_output_loop" not in {issue.code for issue in result.reportable_issues}


def test_static_analyzer_can_match_regex_issue_details():
    snapshot = FakeSession().export_prototype_snapshot()
    snapshot["recipes"]["broken-widget"] = {
        "name": "broken-widget",
        "category": "missing-widget-crafting",
        "ingredients": [
            {"name": "missing-widget-part", "type": "item", "amount": 1},
        ],
        "products": [{"name": "iron-plate", "type": "item", "amount": 1}],
    }
    policy = StaticIssuePolicy.from_options(
        suppress_issue_matches=[
            {
                "code": "recipe_missing_crafting_machine",
                "details_regex": {"category": "^missing-.+-crafting$"},
            }
        ],
        severity_override_matches=[
            {
                "code": "recipe_missing_ingredient_prototype",
                "details_regex": {"ingredients": "^missing-widget-"},
                "severity": "warning",
            }
        ],
    )

    result = StaticAnalyzer(policy).analyze(snapshot)

    broken_issues = [
        issue
        for issue in result.issues
        if issue.details.get("recipe") == "broken-widget"
    ]
    assert [issue.code for issue in broken_issues] == [
        "recipe_missing_ingredient_prototype"
    ]
    assert broken_issues[0].severity == "warning"


def test_static_analyzer_rejects_invalid_regex_issue_details():
    with pytest.raises(ValueError, match="details_regex"):
        StaticIssuePolicy.from_options(
            suppress_issue_matches=[
                {
                    "code": "recipe_missing_crafting_machine",
                    "details_regex": {"category": "["},
                }
            ]
        )


def test_static_analyzer_can_match_boolean_issue_rules():
    policy = StaticIssuePolicy.from_options(
        suppress_issue_matches=[
            {
                "all": [
                    {"code": "recipe_missing_crafting_machine"},
                    {
                        "any": [
                            {"details": {"recipe": "iron-plate"}},
                            {"details": {"recipe": "debug-recipe"}},
                        ]
                    },
                    {"not": {"details_regex": {"category": "^allowed-"}}},
                ]
            }
        ],
        severity_override_matches=[
            {
                "all": [
                    {"code": "item_without_use"},
                    {"not": {"prototype": "copper-plate"}},
                ],
                "severity": "warning",
            }
        ],
    )

    result = StaticAnalyzer(policy).analyze(FakeSession().export_prototype_snapshot())

    assert [issue.code for issue in result.issues] == ["item_without_use"]
    assert result.issues[0].severity == "warning"


def test_static_analyzer_rejects_invalid_boolean_issue_rules():
    with pytest.raises(ValueError, match="all"):
        StaticIssuePolicy.from_options(
            suppress_issue_matches=[
                {
                    "all": {"code": "recipe_missing_crafting_machine"},
                }
            ]
        )


def test_static_analyzer_can_match_policy_by_prototype_name():
    policy = StaticIssuePolicy.from_options(
        suppress_issue_matches=[
            {
                "code": "recipe_missing_crafting_machine",
                "prototype": "iron-plate",
            }
        ],
        severity_override_matches=[
            {
                "prototype": "iron-plate",
                "severity": "warning",
            }
        ],
        min_report_severity="warning",
    )

    result = StaticAnalyzer(policy).analyze(FakeSession().export_prototype_snapshot())

    assert [issue.code for issue in result.issues] == ["item_without_use"]
    assert result.issues[0].severity == "warning"
    assert result.reportable_issues == result.issues


def test_static_analyzer_can_match_policy_by_inline_mod_provenance():
    snapshot = FakeSession().export_prototype_snapshot()
    snapshot["recipes"]["iron-plate"]["source_mod"] = "debug-mod"
    snapshot["items"]["iron-plate"]["source_mod"] = "base"
    policy = StaticIssuePolicy.from_options(
        suppress_issue_matches=[
            {
                "code": "recipe_missing_crafting_machine",
                "mod": "debug-mod",
            }
        ],
        severity_override_matches=[
            {
                "code": "item_without_use",
                "mod": "base",
                "severity": "warning",
            }
        ],
    )

    result = StaticAnalyzer(policy).analyze(snapshot)

    assert [issue.code for issue in result.issues] == ["item_without_use"]
    assert result.issues[0].severity == "warning"


def test_static_analyzer_can_match_policy_by_prototype_mods_map():
    snapshot = FakeSession().export_prototype_snapshot()
    snapshot["prototype_mods"] = {
        "recipes": {"iron-plate": {"source_mod": "recipe-mod"}},
        "items": {"iron-plate": "item-mod"},
    }
    policy = StaticIssuePolicy.from_options(
        suppress_issue_matches=[
            {
                "code": "recipe_missing_crafting_machine",
                "mod": "recipe-mod",
            }
        ],
        severity_override_matches=[
            {
                "code": "item_without_use",
                "mod": "item-mod",
                "severity": "warning",
            }
        ],
    )

    result = StaticAnalyzer(policy).analyze(snapshot)

    assert [issue.code for issue in result.issues] == ["item_without_use"]
    assert result.issues[0].severity == "warning"
