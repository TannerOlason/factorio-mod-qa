from __future__ import annotations

import re
from dataclasses import asdict, dataclass, replace
from typing import Any, Iterable

from prototypes.snapshot import PrototypeSnapshot
from prototypes.validators import ValidationIssue, validate_snapshot


SEVERITY_RANKS = {"info": 0, "warning": 1, "error": 2}


@dataclass(frozen=True)
class StaticIssueMatch:
    code: str | None = None
    prototype: str | None = None
    mod: str | None = None
    details: dict[str, Any] | None = None
    details_contains: dict[str, Any] | None = None
    details_regex: dict[str, str] | None = None
    all_of: tuple["StaticIssueMatch", ...] = ()
    any_of: tuple["StaticIssueMatch", ...] = ()
    not_match: "StaticIssueMatch | None" = None
    severity: str | None = None

    @classmethod
    def from_dict(
        cls, data: dict[str, Any], *, require_severity: bool = False
    ) -> "StaticIssueMatch":
        if not isinstance(data, dict):
            raise ValueError("Static issue match rules must be JSON objects")
        details = data.get("details")
        if details is not None and not isinstance(details, dict):
            raise ValueError("Static issue match rule details must be a JSON object")
        details_contains = data.get("details_contains")
        if details_contains is not None and not isinstance(details_contains, dict):
            raise ValueError(
                "Static issue match rule details_contains must be a JSON object"
            )
        details_regex = data.get("details_regex")
        if details_regex is not None and not isinstance(details_regex, dict):
            raise ValueError("Static issue match rule details_regex must be a JSON object")
        if details_regex:
            for key, pattern in details_regex.items():
                if not isinstance(pattern, str):
                    raise ValueError(
                        "Static issue match rule details_regex values must be strings"
                    )
                try:
                    re.compile(pattern)
                except re.error as exc:
                    raise ValueError(
                        f"Invalid static issue match details_regex for {key}: {exc}"
                    ) from exc
        all_of = _match_rule_list(data.get("all"), key="all")
        any_of = _match_rule_list(data.get("any"), key="any")
        not_rule = data.get("not")
        if not_rule is not None and not isinstance(not_rule, dict):
            raise ValueError("Static issue match rule not must be a JSON object")
        not_match = cls.from_dict(not_rule) if not_rule else None
        if (
            not data.get("code")
            and not data.get("prototype")
            and not data.get("mod")
            and not details
            and not details_contains
            and not details_regex
            and not all_of
            and not any_of
            and not not_match
        ):
            raise ValueError(
                "Static issue match rules require code, prototype, mod, details, details_contains, details_regex, all, any, or not"
            )
        severity = data.get("severity")
        if require_severity and not severity:
            raise ValueError("Severity override match rules require severity")
        if severity is not None:
            severity = str(severity).lower()
        return cls(
            code=data.get("code"),
            prototype=data.get("prototype"),
            mod=data.get("mod"),
            details=details,
            details_contains=details_contains,
            details_regex=details_regex,
            all_of=all_of,
            any_of=any_of,
            not_match=not_match,
            severity=severity,
        )

    def matches(
        self, issue: ValidationIssue, snapshot: PrototypeSnapshot | None = None
    ) -> bool:
        if self.code and issue.code != self.code:
            return False
        if self.prototype and not _details_reference_prototype(
            issue.details,
            self.prototype,
        ):
            return False
        if self.mod and not _issue_references_mod(issue, snapshot, self.mod):
            return False
        if self.details:
            for key, expected in self.details.items():
                if issue.details.get(key) != expected:
                    return False
        if self.details_contains:
            for key, expected in self.details_contains.items():
                if not _detail_contains(issue.details.get(key), expected):
                    return False
        if self.details_regex:
            for key, pattern in self.details_regex.items():
                if not _detail_regex_matches(issue.details.get(key), pattern):
                    return False
        if self.all_of and not all(
            rule.matches(issue, snapshot) for rule in self.all_of
        ):
            return False
        if self.any_of and not any(
            rule.matches(issue, snapshot) for rule in self.any_of
        ):
            return False
        if self.not_match and self.not_match.matches(issue, snapshot):
            return False
        return True


def _match_rule_list(data: Any, *, key: str) -> tuple[StaticIssueMatch, ...]:
    if data is None:
        return ()
    if not isinstance(data, list):
        raise ValueError(f"Static issue match rule {key} must be a JSON array")
    return tuple(StaticIssueMatch.from_dict(rule) for rule in data)


def _detail_contains(actual: Any, expected: Any) -> bool:
    if isinstance(actual, str):
        return str(expected) in actual
    if isinstance(actual, dict):
        if isinstance(expected, dict):
            return all(actual.get(key) == value for key, value in expected.items())
        return expected in actual
    if isinstance(actual, (list, tuple, set)):
        if isinstance(expected, (list, tuple, set)):
            return all(value in actual for value in expected)
        return expected in actual
    return actual == expected


def _detail_regex_matches(actual: Any, pattern: str) -> bool:
    if actual is None:
        return False
    regex = re.compile(pattern)
    if isinstance(actual, str):
        return bool(regex.search(actual))
    if isinstance(actual, dict):
        return any(
            regex.search(str(key)) or _detail_regex_matches(value, pattern)
            for key, value in actual.items()
        )
    if isinstance(actual, (list, tuple, set)):
        return any(_detail_regex_matches(value, pattern) for value in actual)
    return bool(regex.search(str(actual)))


def _details_reference_prototype(details: dict[str, Any], prototype: str) -> bool:
    for value in details.values():
        if value == prototype:
            return True
        if isinstance(value, (list, tuple, set)) and prototype in value:
            return True
    return False


PROTOTYPE_SECTIONS = (
    "recipes",
    "items",
    "fluids",
    "entities",
    "technologies",
    "resources",
    "modules",
    "crafting_categories",
    "resource_categories",
    "tiles",
    "equipment",
    "achievements",
    "surfaces",
)

PROTOTYPE_MOD_KEYS = ("source_mod", "owning_mod", "owner_mod", "mod_name", "mod")
DETAIL_SECTION_HINTS = {
    "recipe": ("recipes",),
    "recipes": ("recipes",),
    "item": ("items",),
    "items": ("items",),
    "fluid": ("fluids",),
    "fluids": ("fluids",),
    "entity": ("entities",),
    "entities": ("entities",),
    "technology": ("technologies",),
    "technologies": ("technologies",),
    "resource": ("resources",),
    "resources": ("resources",),
    "module": ("modules",),
    "modules": ("modules",),
    "tile": ("tiles",),
    "tiles": ("tiles",),
    "equipment": ("equipment",),
    "achievement": ("achievements",),
    "achievements": ("achievements",),
    "surface": ("surfaces",),
    "surfaces": ("surfaces",),
}


def _issue_references_mod(
    issue: ValidationIssue,
    snapshot: PrototypeSnapshot | None,
    mod_name: str,
) -> bool:
    if snapshot is None:
        return False
    for prototype_name, section_names in _details_prototype_candidates(issue.details):
        if _prototype_owner_mod(snapshot, prototype_name, section_names) == mod_name:
            return True
    return False


def _details_prototype_candidates(
    value: Any,
    section_names: tuple[str, ...] | None = None,
) -> set[tuple[str, tuple[str, ...] | None]]:
    if isinstance(value, str):
        return {(value, section_names)}
    if isinstance(value, dict):
        candidates = set()
        for key, child in value.items():
            candidates.update(
                _details_prototype_candidates(
                    child,
                    DETAIL_SECTION_HINTS.get(str(key), section_names),
                )
            )
        return candidates
    if isinstance(value, (list, tuple, set)):
        candidates = set()
        for child in value:
            candidates.update(_details_prototype_candidates(child, section_names))
        return candidates
    return set()


def _prototype_owner_mod(
    snapshot: PrototypeSnapshot,
    prototype_name: str,
    section_names: tuple[str, ...] | None = None,
) -> str | None:
    for section_name in section_names or ():
        owner = _prototype_owner_mod_in_section(snapshot, section_name, prototype_name)
        if owner:
            return owner

    mapped = snapshot.prototype_mods.get(prototype_name)
    if isinstance(mapped, str):
        return mapped
    if isinstance(mapped, dict):
        for key in PROTOTYPE_MOD_KEYS:
            if mapped.get(key):
                return str(mapped[key])

    for section_name in PROTOTYPE_SECTIONS:
        owner = _prototype_owner_mod_in_section(snapshot, section_name, prototype_name)
        if owner:
            return owner
    return None


def _prototype_owner_mod_in_section(
    snapshot: PrototypeSnapshot,
    section_name: str,
    prototype_name: str,
) -> str | None:
    section = getattr(snapshot, section_name)
    prototype = section.get(prototype_name)
    if isinstance(prototype, dict):
        for key in PROTOTYPE_MOD_KEYS:
            if prototype.get(key):
                return str(prototype[key])

    section_map = snapshot.prototype_mods.get(section_name)
    if isinstance(section_map, dict):
        owner = section_map.get(prototype_name)
        if isinstance(owner, str):
            return owner
        if isinstance(owner, dict):
            for key in PROTOTYPE_MOD_KEYS:
                if owner.get(key):
                    return str(owner[key])
    return None


@dataclass(frozen=True)
class StaticIssuePolicy:
    positive_loop_whitelist: set[str]
    suppress_issue_codes: set[str]
    suppress_issue_matches: tuple[StaticIssueMatch, ...]
    severity_overrides: dict[str, str]
    severity_override_matches: tuple[StaticIssueMatch, ...]
    min_report_severity: str = "info"

    @classmethod
    def from_options(
        cls,
        *,
        positive_loop_whitelist: Iterable[str] | None = None,
        suppress_issue_codes: Iterable[str] | None = None,
        suppress_issue_matches: Iterable[dict[str, Any]] | None = None,
        severity_overrides: dict[str, str] | None = None,
        severity_override_matches: Iterable[dict[str, Any]] | None = None,
        min_report_severity: str = "info",
    ) -> "StaticIssuePolicy":
        return cls(
            positive_loop_whitelist=set(positive_loop_whitelist or []),
            suppress_issue_codes=set(suppress_issue_codes or []),
            suppress_issue_matches=tuple(
                StaticIssueMatch.from_dict(rule)
                for rule in suppress_issue_matches or []
            ),
            severity_overrides=dict(severity_overrides or {}),
            severity_override_matches=tuple(
                StaticIssueMatch.from_dict(rule, require_severity=True)
                for rule in severity_override_matches or []
            ),
            min_report_severity=min_report_severity,
        )

    def apply(
        self,
        issues: list[ValidationIssue],
        snapshot: PrototypeSnapshot | None = None,
    ) -> list[ValidationIssue]:
        filtered = []
        for issue in issues:
            if issue.code in self.suppress_issue_codes:
                continue
            if any(
                rule.matches(issue, snapshot) for rule in self.suppress_issue_matches
            ):
                continue

            override = self.severity_overrides.get(issue.code)
            if override:
                normalized = override.lower()
                if normalized in {"ignore", "off", "suppress", "suppressed"}:
                    continue
                issue = replace(issue, severity=normalized)
            for rule in self.severity_override_matches:
                if not rule.matches(issue, snapshot) or not rule.severity:
                    continue
                if rule.severity in {"ignore", "off", "suppress", "suppressed"}:
                    issue = None
                    break
                issue = replace(issue, severity=rule.severity)

            if issue is None:
                continue

            filtered.append(issue)
        return filtered

    def is_reportable(self, severity: str) -> bool:
        minimum = SEVERITY_RANKS.get(self.min_report_severity, 0)
        return SEVERITY_RANKS.get(severity, 0) >= minimum


@dataclass(frozen=True)
class StaticAnalysisResult:
    snapshot: PrototypeSnapshot
    raw_issues: list[ValidationIssue]
    issues: list[ValidationIssue]
    reportable_issues: list[ValidationIssue]

    def issues_as_dicts(self) -> list[dict[str, Any]]:
        return [asdict(issue) for issue in self.issues]

    def summary_counts(self) -> dict[str, int]:
        return {
            "raw_static_issue_count": len(self.raw_issues),
            "static_issue_count": len(self.issues),
            "reportable_static_issue_count": len(self.reportable_issues),
            "suppressed_static_issue_count": len(self.raw_issues) - len(self.issues),
        }


class StaticAnalyzer:
    def __init__(self, policy: StaticIssuePolicy):
        self.policy = policy

    def analyze(self, snapshot_data: dict[str, Any]) -> StaticAnalysisResult:
        snapshot = PrototypeSnapshot.from_dict(snapshot_data)
        raw_issues = validate_snapshot(snapshot, self.policy.positive_loop_whitelist)
        issues = self.policy.apply(raw_issues, snapshot)
        reportable_issues = [
            issue for issue in issues if self.policy.is_reportable(issue.severity)
        ]
        return StaticAnalysisResult(
            snapshot=snapshot,
            raw_issues=raw_issues,
            issues=issues,
            reportable_issues=reportable_issues,
        )
