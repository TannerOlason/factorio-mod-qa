from __future__ import annotations

import json
from pathlib import Path
from typing import Any, Iterable

from mod_qa_agent.bug_detector import BugDetector
from mod_qa_agent.mod_source import list_mod_sources, summarize_mod_sources
from mod_qa_agent.novelty_archive import NoveltyArchive
from mod_qa_agent.report_writer import ReportWriter
from mod_qa_agent.static_analyzer import StaticAnalyzer, StaticIssuePolicy
from mod_qa_agent.trace_generator import TraceGenerator
from mod_qa_agent.trace_runner import TraceRunner


class FuzzOrchestrator:
    def __init__(
        self,
        *,
        session,
        run_dir: str | Path,
        reports_dir: str | Path,
        seed: int | None,
        goals: int = 20,
        max_traces: int | None = None,
        llm_planning: bool = False,
        native_saves: bool = False,
        mods_path: str | Path | None = None,
        mutations: int = 0,
        positive_loop_whitelist: Iterable[str] | None = None,
        suppress_issue_codes: Iterable[str] | None = None,
        suppress_issue_matches: Iterable[dict[str, Any]] | None = None,
        severity_overrides: dict[str, str] | None = None,
        severity_override_matches: Iterable[dict[str, Any]] | None = None,
        min_report_severity: str = "info",
        validate_only: bool = False,
    ):
        self.session = session
        self.run_dir = Path(run_dir)
        self.reports_dir = Path(reports_dir)
        self.seed = seed
        self.goals = goals
        self.max_traces = max_traces or goals
        self.llm_planning = llm_planning
        self.mods_path = Path(mods_path) if mods_path else None
        self.mutations = max(0, mutations)
        self.static_policy = StaticIssuePolicy.from_options(
            positive_loop_whitelist=positive_loop_whitelist,
            suppress_issue_codes=suppress_issue_codes,
            suppress_issue_matches=suppress_issue_matches,
            severity_overrides=severity_overrides,
            severity_override_matches=severity_override_matches,
            min_report_severity=min_report_severity,
        )
        self.validate_only = validate_only
        save_callback = getattr(session, "save_native", None) if native_saves else None
        self.archive = NoveltyArchive(self.run_dir / "archive", save_callback)
        self.detector = BugDetector()

    def run(self) -> dict:
        self.run_dir.mkdir(parents=True, exist_ok=True)
        snapshot_data = self.session.export_prototype_snapshot()
        snapshot_path = self.run_dir / "prototype_snapshot.json"
        snapshot_path.write_text(
            json.dumps(snapshot_data, indent=2, sort_keys=True),
            encoding="utf-8",
        )

        static_result = StaticAnalyzer(self.static_policy).analyze(snapshot_data)
        snapshot = static_result.snapshot
        issues = static_result.issues
        issues_path = self.run_dir / "static_validation_issues.json"
        issues_path.write_text(
            json.dumps(
                static_result.issues_as_dicts(),
                indent=2,
                sort_keys=True,
            ),
            encoding="utf-8",
        )

        mod_source_summary = self._mod_source_summary()
        mod_source_status = self._mod_source_status(
            mod_source_summary,
            snapshot.active_mods,
        )
        report_writer = ReportWriter(
            self.reports_dir,
            mod_list=snapshot.active_mods,
            factorio_version=snapshot.factorio_version,
            mod_sources=mod_source_summary,
        )

        reports: list[Path] = []
        for report in self.detector.from_validation_issues(
            static_result.reportable_issues
        ):
            reports.append(report_writer.write(report, seed=self.seed))

        traces = []
        mutated_traces = []
        trace_result = None
        if not self.validate_only:
            trace_generator = TraceGenerator(
                goals=self.goals,
                max_traces=self.max_traces,
                seed=self.seed,
                llm_planning=self.llm_planning,
                mod_sources=mod_source_summary,
                mutations=0,
            )
            traces = trace_generator.generate(issues, snapshot)

        trace_runner = TraceRunner(
            session=self.session,
            archive=self.archive,
            detector=self.detector,
            report_writer=report_writer,
            seed=self.seed,
        )
        trace_result = trace_runner.run(traces)
        reports.extend(trace_result.reports)

        if not self.validate_only and self.mutations > 0:
            remaining_trace_budget = max(0, self.max_traces - len(traces))
            mutation_count = min(self.mutations, remaining_trace_budget)
            mutated_traces = trace_generator.generate_feedback_mutations(
                trace_result.executions,
                mutation_count,
            )
            mutation_result = trace_runner.run(mutated_traces)
            trace_result.executions.extend(mutation_result.executions)
            trace_result.reports.extend(mutation_result.reports)
            reports.extend(mutation_result.reports)

        summary = {
            "run_dir": str(self.run_dir),
            "snapshot": str(snapshot_path),
            **static_result.summary_counts(),
            "trace_count": len(traces) + len(mutated_traces),
            "mutated_trace_count": len(mutated_traces),
            "mod_source_count": len(mod_source_summary),
            **mod_source_status,
            "validate_only": self.validate_only,
            "runtime_error_count": trace_result.runtime_error_count,
            "native_save_count": trace_result.native_save_count,
            "native_save_paths": [
                execution.native_save_path
                for execution in trace_result.executions
                if execution.native_save_path
            ],
            "reports": [str(path) for path in reports],
        }
        (self.run_dir / "summary.json").write_text(
            json.dumps(summary, indent=2, sort_keys=True),
            encoding="utf-8",
        )
        return summary

    def _mod_source_summary(self) -> list[dict[str, Any]]:
        if self.mods_path is None:
            return []
        return summarize_mod_sources(list_mod_sources(self.mods_path))

    def _mod_source_status(
        self,
        mod_source_summary: list[dict[str, Any]],
        active_mods: dict[str, str],
    ) -> dict[str, Any]:
        if not mod_source_summary:
            return {
                "mod_sources_not_loaded": [],
                "active_mods_without_source": [],
                "mod_source_warning": None,
            }

        source_names = {
            str(mod["name"])
            for mod in mod_source_summary
            if mod.get("name") is not None
        }
        active_names = set(active_mods)
        not_loaded = sorted(source_names - active_names)
        without_source = sorted(active_names - source_names - {"base"})
        warning = None
        if not_loaded:
            warning = (
                "Some mods from --mods-path were not present in the live "
                "Factorio active_mods snapshot. Reports may reflect the loaded "
                "server state rather than those local mod sources."
            )

        return {
            "mod_sources_not_loaded": not_loaded,
            "active_mods_without_source": without_source,
            "mod_source_warning": warning,
        }
