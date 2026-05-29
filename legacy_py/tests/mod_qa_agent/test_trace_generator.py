import pytest

from prototypes.snapshot import PrototypeSnapshot
from prototypes.validators import ValidationIssue

from mod_qa_agent.action_trace import ActionStep, ActionTrace, TraceExecution
from mod_qa_agent.trace_generator import TraceGenerator

pytestmark = pytest.mark.no_factorio


def make_snapshot():
    return PrototypeSnapshot.from_dict(
        {
            "schema_version": 1,
            "factorio_version": "2.0.73",
            "active_mods": {"base": "2.0.73"},
            "recipes": {
                "copper-plate": {},
                "iron-plate": {},
            },
            "entities": {},
        }
    )


def make_ticking_snapshot():
    data = make_snapshot().to_dict()
    data["active_mods"] = {"base": "2.0.73", "qa-broken-mod": "0.1.0"}
    data["entities"] = {
        "qa-ticking-machine": {
            "name": "qa-ticking-machine",
            "source_mod": "qa-broken-mod",
        }
    }
    return PrototypeSnapshot.from_dict(data)


def make_ticking_snapshot_without_source_mod():
    data = make_ticking_snapshot().to_dict()
    data["entities"]["qa-ticking-machine"].pop("source_mod")
    return PrototypeSnapshot.from_dict(data)


def make_issue(code="recipe_missing_crafting_machine"):
    return ValidationIssue(
        code=code,
        severity="error",
        title=f"Issue {code}",
        details={"recipe": "iron-plate"},
    )


def test_trace_generator_prefers_static_issue_goals_then_recipe_probes():
    traces = TraceGenerator(
        goals=3,
        max_traces=3,
        seed=123,
    ).generate([make_issue()], make_snapshot())

    assert [trace.trace_id for trace in traces] == [
        "static-goal-0001",
        "recipe-probe-0002",
        "recipe-probe-0003",
    ]
    assert traces[0].steps[0].metadata["issue_code"] == (
        "recipe_missing_crafting_machine"
    )
    assert traces[1].steps[0].metadata["recipe"] == "copper-plate"
    assert traces[2].steps[0].metadata["recipe"] == "iron-plate"


def test_trace_generator_prioritizes_script_stress_for_ticking_mod_entities():
    traces = TraceGenerator(
        goals=2,
        max_traces=2,
        seed=123,
    ).generate([make_issue()], make_ticking_snapshot())

    assert traces[0].trace_id == "script-stress-0001"
    assert traces[0].steps[0].kind == "script_stress_probe"
    assert traces[0].steps[0].metadata == {
        "entity": "qa-ticking-machine",
        "source_mod": "qa-broken-mod",
    }
    assert "qa-ticking-machine" in traces[0].steps[0].code
    assert "game.players[1]" not in traces[0].steps[0].code
    assert "game.surfaces[1]" in traces[0].steps[0].code
    assert "reset_script_event_counts" in traces[0].steps[0].code
    assert traces[1].trace_id == "static-goal-0001"


def test_trace_generator_uses_ticking_name_when_factorio_omits_source_mod():
    traces = TraceGenerator(
        goals=1,
        max_traces=1,
        seed=123,
    ).generate([make_issue()], make_ticking_snapshot_without_source_mod())

    assert traces[0].trace_id == "script-stress-0001"
    assert traces[0].steps[0].metadata == {
        "entity": "qa-ticking-machine",
        "source_mod": "unknown",
    }


def test_trace_generator_does_not_script_stress_base_only_snapshots():
    data = make_snapshot().to_dict()
    data["entities"] = {"script-trigger": {"name": "script-trigger"}}

    traces = TraceGenerator(
        goals=1,
        max_traces=1,
        seed=123,
    ).generate([make_issue()], PrototypeSnapshot.from_dict(data))

    assert traces[0].trace_id == "static-goal-0001"


def test_trace_generator_does_not_match_tick_inside_other_words():
    data = make_snapshot().to_dict()
    data["active_mods"] = {"base": "2.0.73", "qa-broken-mod": "0.1.0"}
    data["entities"] = {
        "acid-sticker-behemoth": {"name": "acid-sticker-behemoth"},
        "qa-ticking-machine": {"name": "qa-ticking-machine"},
    }

    traces = TraceGenerator(
        goals=1,
        max_traces=1,
        seed=123,
    ).generate([make_issue()], PrototypeSnapshot.from_dict(data))

    assert traces[0].steps[0].metadata["entity"] == "qa-ticking-machine"


def test_trace_generator_appends_mutations_before_truncating():
    traces = TraceGenerator(
        goals=1,
        max_traces=2,
        seed=123,
        mutations=1,
    ).generate([make_issue()], make_snapshot())

    assert [trace.trace_id for trace in traces] == [
        "static-goal-0001",
        "static-goal-0001-mut-0001",
    ]
    assert traces[1].steps[-1].kind == "wait"


def test_trace_generator_keeps_llm_planning_optional():
    disabled = TraceGenerator(
        goals=1,
        max_traces=3,
        seed=123,
        llm_planning=False,
    ).generate([make_issue()], make_snapshot())
    enabled = TraceGenerator(
        goals=1,
        max_traces=3,
        seed=123,
        llm_planning=True,
    ).generate([make_issue()], make_snapshot())

    assert [trace.trace_id for trace in disabled] == ["static-goal-0001"]
    assert [trace.trace_id for trace in enabled] == [
        "static-goal-0001",
        "llm-placeholder-0001",
    ]


def test_trace_generator_passes_mod_source_context_to_llm_placeholder():
    traces = TraceGenerator(
        goals=1,
        max_traces=3,
        seed=123,
        llm_planning=True,
        mod_sources=[
            {
                "name": "qa-broken-mod",
                "version": "0.1.0",
                "dependencies": ["base >= 2.0.0"],
                "entrypoints": ["data.lua", "control.lua"],
                "path": "/ignored",
            }
        ],
    ).generate([make_issue()], make_snapshot())

    llm_step = traces[1].steps[0]

    assert llm_step.metadata["mod_sources"] == [
        {
            "name": "qa-broken-mod",
            "version": "0.1.0",
            "dependencies": ["base >= 2.0.0"],
            "entrypoints": ["data.lua", "control.lua"],
        }
    ]
    assert "mod source context" in llm_step.code


def test_trace_generator_passes_issue_context_to_llm_placeholder():
    issue = make_issue(code="technology_unlocks_missing_recipe")
    issue.details["recipes"] = ["missing-recipe"]

    traces = TraceGenerator(
        goals=1,
        max_traces=3,
        seed=123,
        llm_planning=True,
    ).generate([issue], make_snapshot())

    llm_step = traces[1].steps[0]

    assert llm_step.metadata["issue"] == {
        "code": "technology_unlocks_missing_recipe",
        "severity": "error",
        "title": "Issue technology_unlocks_missing_recipe",
        "details": {
            "recipe": "iron-plate",
            "recipes": ["missing-recipe"],
        },
    }
    assert "issue context" in llm_step.code


def test_trace_generator_feedback_mutations_prefer_errors():
    ok_trace = ActionTrace(
        trace_id="ok-0001",
        goal="OK trace",
        steps=[ActionStep(kind="probe", code="print('ok')")],
    )
    error_trace = ActionTrace(
        trace_id="error-0001",
        goal="Error trace",
        steps=[ActionStep(kind="probe", code="missing_name")],
    )
    executions = [
        TraceExecution(
            trace=ok_trace,
            response="ok",
            error=False,
            signature={"items_seen": ["iron-plate", "copper-plate"]},
        ),
        TraceExecution(
            trace=error_trace,
            response="NameError",
            error=True,
            signature={"items_seen": ["iron-plate"]},
        ),
    ]

    traces = TraceGenerator(goals=1, max_traces=3, seed=123).generate_feedback_mutations(
        executions,
        count=1,
    )

    assert [trace.trace_id for trace in traces] == ["error-0001-mut-0001"]
    assert traces[0].steps[-1].metadata["source_trace_id"] == "error-0001"
    assert traces[0].steps[-1].metadata["source_error"] is True


def test_trace_generator_feedback_mutations_prefer_novelty_reasons():
    broad_trace = ActionTrace(
        trace_id="broad-0001",
        goal="Broad signature trace",
        steps=[ActionStep(kind="probe", code="print('broad')")],
    )
    novel_trace = ActionTrace(
        trace_id="novel-0001",
        goal="Novel trace",
        steps=[ActionStep(kind="probe", code="print('novel')")],
    )
    executions = [
        TraceExecution(
            trace=broad_trace,
            response="ok",
            error=False,
            signature={"items_seen": ["iron-plate", "copper-plate", "steel-plate"]},
        ),
        TraceExecution(
            trace=novel_trace,
            response="ok",
            error=False,
            signature={"items_seen": ["iron-plate"]},
            novelty_reasons=["surface: vulcanus", "fluids_seen: molten-iron"],
        ),
    ]

    traces = TraceGenerator(goals=1, max_traces=3, seed=123).generate_feedback_mutations(
        executions,
        count=1,
    )

    assert [trace.trace_id for trace in traces] == ["novel-0001-mut-0001"]
    assert traces[0].steps[-1].metadata["source_novelty_reasons"] == [
        "surface: vulcanus",
        "fluids_seen: molten-iron",
    ]
