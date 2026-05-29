from __future__ import annotations

from prototypes.validators import ValidationIssue

from mod_qa_agent.action_trace import ActionStep, ActionTrace


class LLMPlanner:
    """Optional planning hook. Milestone 1 intentionally stays deterministic."""

    def __init__(
        self,
        enabled: bool = False,
        mod_sources: list[dict[str, object]] | None = None,
    ):
        self.enabled = enabled
        self.mod_sources = mod_sources or []

    def propose_traces(
        self,
        issues: list[ValidationIssue],
        limit: int,
        seed: int | None,
    ) -> list[ActionTrace]:
        if not self.enabled:
            return []
        mod_context = [
            {
                "name": source.get("name"),
                "version": source.get("version"),
                "dependencies": source.get("dependencies", []),
                "entrypoints": source.get("entrypoints", []),
            }
            for source in self.mod_sources
        ]
        traces = []
        for index, issue in enumerate(issues[:limit], start=1):
            issue_context = {
                "code": issue.code,
                "severity": issue.severity,
                "title": issue.title,
                "details": dict(issue.details),
            }
            traces.append(
                ActionTrace(
                    trace_id=f"llm-placeholder-{index:04d}",
                    goal=f"Investigate {issue.title}",
                    seed=seed,
                    steps=[
                        ActionStep(
                            kind="inspect",
                            code=(
                                f"print({issue.title!r})\n"
                                f"print('issue context:', {issue_context!r})\n"
                                f"print('mod source context:', {mod_context!r})"
                            ),
                            metadata={
                                "issue_code": issue.code,
                                "issue": issue_context,
                                "mod_sources": mod_context,
                            },
                        )
                    ],
                )
            )
        return traces
