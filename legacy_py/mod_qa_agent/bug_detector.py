from __future__ import annotations

from dataclasses import asdict, dataclass, field
from typing import Any

from prototypes.validators import ValidationIssue

from mod_qa_agent.action_trace import TraceExecution


@dataclass(frozen=True)
class BugReport:
    issue_id: str
    title: str
    severity: str
    category: str
    details: dict[str, Any] = field(default_factory=dict)


class BugDetector:
    script_event_threshold = 250

    def from_validation_issues(
        self, issues: list[ValidationIssue], limit: int | None = None
    ) -> list[BugReport]:
        reports = []
        selected = issues[:limit] if limit else issues
        for index, issue in enumerate(selected, start=1):
            reports.append(
                BugReport(
                    issue_id=f"static-{index:04d}-{issue.code}",
                    title=issue.title,
                    severity=issue.severity,
                    category=issue.code,
                    details=asdict(issue),
                )
            )
        return reports

    def from_execution(self, execution: TraceExecution) -> list[BugReport]:
        reports = []
        if execution.error:
            reports.append(
                BugReport(
                    issue_id=f"runtime-{execution.trace.trace_id}",
                    title=f"Trace {execution.trace.trace_id} produced an execution error",
                    severity="error",
                    category="runtime_error",
                    details={
                        "goal": execution.trace.goal,
                        "response": execution.response,
                        "duration": execution.duration,
                        "error_details": (
                            asdict(execution.error_details)
                            if execution.error_details
                            else None
                        ),
                    },
                )
            )

        reports.extend(self._script_event_reports(execution))
        return reports

    def _script_event_reports(self, execution: TraceExecution) -> list[BugReport]:
        if not any(
            step.kind == "script_stress_probe" for step in execution.trace.steps
        ):
            return []

        counters = _script_event_counters(execution.signature)
        reports = []
        for name, count in counters.items():
            if count < self.script_event_threshold:
                continue
            reports.append(
                BugReport(
                    issue_id=f"runtime-{execution.trace.trace_id}-script-events-{name}",
                    title=(
                        f"Trace {execution.trace.trace_id} produced high script "
                        f"event growth in {name}"
                    ),
                    severity="warning",
                    category="script_event_growth",
                    details={
                        "goal": execution.trace.goal,
                        "counter": name,
                        "count": count,
                        "threshold": self.script_event_threshold,
                        "signature": dict(execution.signature),
                    },
                )
            )
        return reports


def _script_event_counters(signature: dict[str, Any]) -> dict[str, int]:
    counters = {}
    for flag in signature.get("script_event_state") or []:
        if not isinstance(flag, str) or not flag.startswith("script_events."):
            continue
        name, _, raw_count = flag.partition(":")
        if not raw_count:
            continue
        try:
            counters[name.removeprefix("script_events.")] = int(float(raw_count))
        except ValueError:
            continue
    return counters
