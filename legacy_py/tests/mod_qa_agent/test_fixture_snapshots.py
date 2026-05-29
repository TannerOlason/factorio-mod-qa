import json
from pathlib import Path

import pytest

from mod_qa_agent.static_analyzer import StaticAnalyzer, StaticIssuePolicy

pytestmark = pytest.mark.no_factorio

FIXTURE_SNAPSHOT = (
    Path(__file__).parents[3]
    / "fixtures"
    / "prototype_snapshots"
    / "qa_broken_mod.json"
)


def test_broken_mod_snapshot_exercises_static_validators():
    snapshot = json.loads(FIXTURE_SNAPSHOT.read_text(encoding="utf-8"))

    result = StaticAnalyzer(StaticIssuePolicy.from_options()).analyze(snapshot)
    codes = {issue.code for issue in result.issues}

    assert "recipe_missing_ingredient_prototype" in codes
    assert "recipe_missing_crafting_machine" in codes
    assert "technology_missing_prerequisite" in codes
    assert "technology_unlocks_missing_recipe" in codes
    assert "technology_unreachable_science_pack" in codes
    assert "positive_output_loop" in codes
    assert result.snapshot.recipes["qa-positive-loop"]["source_mod"] == "qa-broken-mod"


def test_broken_mod_snapshot_supports_mod_scoped_policy():
    snapshot = json.loads(FIXTURE_SNAPSHOT.read_text(encoding="utf-8"))
    policy = StaticIssuePolicy.from_options(
        suppress_issue_matches=[
            {
                "code": "recipe_missing_crafting_machine",
                "mod": "qa-broken-mod",
            }
        ]
    )

    result = StaticAnalyzer(policy).analyze(snapshot)

    assert "recipe_missing_crafting_machine" not in {
        issue.code for issue in result.issues
    }
