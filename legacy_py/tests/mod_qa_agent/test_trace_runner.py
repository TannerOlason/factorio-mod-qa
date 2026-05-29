import json

import pytest

from mod_qa_agent.action_trace import (
    ActionStep,
    ActionTrace,
    ExecutionError,
    TraceExecution,
)
from mod_qa_agent.bug_detector import BugDetector
from mod_qa_agent.novelty_archive import NoveltyArchive
from mod_qa_agent.report_writer import ReportWriter
from mod_qa_agent.trace_runner import TraceRunner

pytestmark = pytest.mark.no_factorio


def make_trace(trace_id="probe-0001"):
    return ActionTrace(
        trace_id=trace_id,
        goal="Probe trace runner",
        seed=123,
        steps=[ActionStep(kind="probe", code="print('ok')")],
    )


class TraceSession:
    def __init__(self, execution):
        self.execution = execution
        self.executed_trace_ids = []

    def execute_trace(self, trace):
        self.executed_trace_ids.append(trace.trace_id)
        self.execution.trace = trace
        return self.execution


def make_runner(tmp_path, session):
    return TraceRunner(
        session=session,
        archive=NoveltyArchive(tmp_path / "archive"),
        detector=BugDetector(),
        report_writer=ReportWriter(
            tmp_path / "reports",
            mod_list={"base": "2.0.73"},
            factorio_version="2.0.73",
        ),
        seed=123,
    )


def test_trace_runner_archives_novel_state_without_report(tmp_path):
    trace = make_trace()
    session = TraceSession(
        TraceExecution(
            trace=trace,
            response="ok",
            error=False,
            game_state_raw=json.dumps({"inventories": []}),
            signature={
                "techs": [],
                "items_seen": ["iron-plate"],
                "recipes_used": ["iron-plate"],
                "entities_built": [],
                "fluids_seen": [],
                "surface": "nauvis",
                "milestone_flags": [],
            },
        )
    )

    result = make_runner(tmp_path, session).run([trace])

    assert session.executed_trace_ids == ["probe-0001"]
    assert result.executions[0].response == "ok"
    assert result.runtime_error_count == 0
    assert result.native_save_count == 0
    assert result.executions[0].novelty_reasons == [
        "items_seen: iron-plate",
        "recipes_used: iron-plate",
        "surface: nauvis",
    ]
    assert result.reports == []
    assert (tmp_path / "archive" / "state_000001.json").exists()
    assert not list((tmp_path / "reports").glob("*.md"))


def test_trace_runner_reports_runtime_error(tmp_path):
    trace = make_trace("error-0001")
    session = TraceSession(
        TraceExecution(
            trace=trace,
            response="Lua exception: bad signal",
            error=True,
            duration=0.25,
            error_details=ExecutionError(
                kind="RuntimeError",
                message="Lua exception: bad signal",
                source="factorio_instance",
            ),
            game_state_raw=json.dumps({"inventories": []}),
            signature={
                "techs": [],
                "items_seen": [],
                "recipes_used": [],
                "entities_built": [],
                "fluids_seen": [],
                "surface": "nauvis",
                "milestone_flags": [],
            },
        )
    )

    result = make_runner(tmp_path, session).run([trace])

    assert result.runtime_error_count == 1
    assert len(result.reports) == 1
    assert (tmp_path / "archive" / "state_000001.json").exists()
    report_text = result.reports[0].read_text()
    assert "Trace error-0001 produced an execution error" in report_text
    assert "- Error kind: RuntimeError" in report_text
    assert "- Error source: factorio_instance" in report_text
    assert "Lua exception: bad signal" in report_text
    assert '"kind": "RuntimeError"' in report_text


def test_trace_runner_reports_script_event_growth(tmp_path):
    trace = ActionTrace(
        trace_id="script-stress-0001",
        goal="Stress script scaling",
        seed=123,
        steps=[ActionStep(kind="script_stress_probe", code="print('ok')")],
    )
    session = TraceSession(
        TraceExecution(
            trace=trace,
            response="script stress probe placed qa-ticking-machine",
            error=False,
            duration=3.0,
            game_state_raw=json.dumps({"inventories": []}),
            signature={
                "techs": [],
                "items_seen": [],
                "recipes_used": [],
                "entities_built": ["qa-ticking-machine"],
                "fluids_seen": [],
                "surface": "nauvis",
                "script_event_state": ["script_events.mod_qa_events:288"],
                "milestone_flags": ["script_events.mod_qa_events:288"],
            },
        )
    )

    result = make_runner(tmp_path, session).run([trace])

    assert result.runtime_error_count == 0
    assert len(result.reports) == 1
    report_text = result.reports[0].read_text()
    assert "high script event growth" in report_text
    assert "script_event_growth" in report_text
    assert '"counter": "mod_qa_events"' in report_text
    assert '"count": 288' in report_text


def test_trace_runner_does_not_report_script_growth_on_static_trace(tmp_path):
    trace = make_trace("static-goal-0001")
    session = TraceSession(
        TraceExecution(
            trace=trace,
            response="ok",
            error=False,
            signature={
                "surface": "nauvis",
                "script_event_state": ["script_events.mod_qa_events:288"],
            },
        )
    )

    result = make_runner(tmp_path, session).run([trace])

    assert result.reports == []
