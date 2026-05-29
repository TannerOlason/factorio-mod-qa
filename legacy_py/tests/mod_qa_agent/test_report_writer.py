import pytest

from mod_qa_agent.bug_detector import BugReport
from mod_qa_agent.report_writer import ReportWriter

pytestmark = pytest.mark.no_factorio


def test_report_writer_surfaces_native_save_path(tmp_path):
    report = BugReport(
        issue_id="runtime-probe-0001",
        title="Runtime failure",
        severity="error",
        category="runtime_error",
    )
    writer = ReportWriter(
        tmp_path / "reports",
        mod_list={"base": "2.0.73"},
        factorio_version="2.0.73",
    )

    path = writer.write(
        report,
        seed=123,
        extra={"native_save_path": "/tmp/mod_qa_state_000001.zip"},
    )

    report_text = path.read_text(encoding="utf-8")
    assert "- Native save: /tmp/mod_qa_state_000001.zip" in report_text


def test_report_writer_sanitizes_issue_id_filename(tmp_path):
    report = BugReport(
        issue_id="../runtime/probe 1",
        title="Runtime failure",
        severity="error",
        category="runtime_error",
    )
    reports_dir = tmp_path / "reports"
    writer = ReportWriter(
        reports_dir,
        mod_list={"base": "2.0.73"},
        factorio_version="2.0.73",
    )

    path = writer.write(report, seed=None)

    assert path.parent == reports_dir
    assert path.name == "runtime_probe_1.md"
    assert path.exists()
    assert not (tmp_path / "runtime").exists()
