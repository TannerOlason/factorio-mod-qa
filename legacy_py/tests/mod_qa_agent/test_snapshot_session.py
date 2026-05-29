import json

import pytest

from mod_qa_agent.snapshot_session import SnapshotSession

pytestmark = pytest.mark.no_factorio


def test_snapshot_session_loads_snapshot(tmp_path):
    snapshot_path = tmp_path / "prototype_snapshot.json"
    snapshot_path.write_text(
        json.dumps(
            {
                "schema_version": 1,
                "factorio_version": "2.0.73",
                "active_mods": {"base": "2.0.73"},
            }
        ),
        encoding="utf-8",
    )

    snapshot = SnapshotSession(snapshot_path).export_prototype_snapshot()

    assert snapshot["active_mods"] == {"base": "2.0.73"}


def test_snapshot_session_rejects_trace_execution(tmp_path):
    snapshot_path = tmp_path / "prototype_snapshot.json"
    snapshot_path.write_text("{}", encoding="utf-8")

    with pytest.raises(RuntimeError, match="cannot execute traces"):
        SnapshotSession(snapshot_path).execute_trace(None)
