from __future__ import annotations

from dataclasses import asdict, dataclass, field
from pathlib import Path

from mod_qa_agent.action_trace import ActionTrace, TraceExecution
from mod_qa_agent.bug_detector import BugDetector
from mod_qa_agent.novelty_archive import NoveltyArchive
from mod_qa_agent.report_writer import ReportWriter


@dataclass
class TraceRunResult:
    executions: list[TraceExecution] = field(default_factory=list)
    reports: list[Path] = field(default_factory=list)

    @property
    def runtime_error_count(self) -> int:
        return sum(1 for execution in self.executions if execution.error)

    @property
    def native_save_count(self) -> int:
        return sum(1 for execution in self.executions if execution.native_save_path)


class TraceRunner:
    def __init__(
        self,
        *,
        session,
        archive: NoveltyArchive,
        detector: BugDetector,
        report_writer: ReportWriter,
        seed: int | None,
    ):
        self.session = session
        self.archive = archive
        self.detector = detector
        self.report_writer = report_writer
        self.seed = seed

    def run(self, traces: list[ActionTrace]) -> TraceRunResult:
        result = TraceRunResult()
        for trace in traces:
            execution = self.session.execute_trace(trace)
            result.executions.append(execution)
            archive_path = self.archive.archive_if_interesting(execution)
            for report in self.detector.from_execution(execution):
                result.reports.append(
                    self.report_writer.write(
                        report,
                        seed=self.seed,
                        trace=trace,
                        archive_path=archive_path,
                        extra={
                            "response": execution.response,
                            "error_details": (
                                asdict(execution.error_details)
                                if execution.error_details
                                else None
                            ),
                            "native_save_path": execution.native_save_path,
                        },
                    )
                )
        return result
