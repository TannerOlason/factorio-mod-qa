from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

from mod_qa_agent.action_trace import ActionTrace
from mod_qa_agent.bug_detector import BugReport


def _report_filename(issue_id: str) -> str:
    slug = re.sub(r"[^A-Za-z0-9_.-]+", "_", issue_id).strip("._-")
    return f"{slug or 'issue'}.md"


class ReportWriter:
    def __init__(
        self,
        reports_dir: str | Path,
        mod_list: dict[str, str],
        factorio_version: str | None,
        mod_sources: list[dict[str, Any]] | None = None,
    ):
        self.reports_dir = Path(reports_dir)
        self.reports_dir.mkdir(parents=True, exist_ok=True)
        self.mod_list = mod_list
        self.factorio_version = factorio_version
        self.mod_sources = mod_sources or []

    def write(
        self,
        report: BugReport,
        *,
        seed: int | None,
        trace: ActionTrace | None = None,
        archive_path: str | Path | None = None,
        extra: dict[str, Any] | None = None,
    ) -> Path:
        path = self.reports_dir / _report_filename(report.issue_id)
        body = [
            f"# {report.title}",
            "",
            f"- Severity: {report.severity}",
            f"- Category: {report.category}",
            f"- Factorio version: {self.factorio_version or 'unknown'}",
            f"- Seed: {seed if seed is not None else 'unknown'}",
            f"- Archive state: {archive_path or 'not archived'}",
            *self._extra_summary_lines(extra or {}),
            "",
            "## Mod List",
            "",
            "```json",
            json.dumps(self.mod_list, indent=2, sort_keys=True),
            "```",
            "",
            *self._mod_source_section(),
            "## Reproduction Trace",
            "",
            "```jsonl",
            trace.to_jsonl().rstrip() if trace else "",
            "```",
            "",
            "## Expected Behavior",
            "",
            "The prototype graph and scripted action trace should be internally consistent and execute without errors.",
            "",
            "## Actual Behavior",
            "",
            report.title,
            "",
            "## Relevant Data",
            "",
            "```json",
            json.dumps(
                {"details": report.details, "extra": extra or {}},
                indent=2,
                sort_keys=True,
            ),
            "```",
        ]
        path.write_text("\n".join(body) + "\n", encoding="utf-8")
        return path

    def _extra_summary_lines(self, extra: dict[str, Any]) -> list[str]:
        lines = []
        native_save_path = extra.get("native_save_path")
        if native_save_path:
            lines.append(f"- Native save: {native_save_path}")

        error_details = extra.get("error_details")
        if isinstance(error_details, dict):
            if error_details.get("kind"):
                lines.append(f"- Error kind: {error_details['kind']}")
            if error_details.get("source"):
                lines.append(f"- Error source: {error_details['source']}")
        return lines

    def _mod_source_section(self) -> list[str]:
        if not self.mod_sources:
            return []
        return [
            "## Mod Source Summary",
            "",
            "```json",
            json.dumps(self.mod_sources, indent=2, sort_keys=True),
            "```",
            "",
        ]
