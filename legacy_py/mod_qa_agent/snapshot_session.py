from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from mod_qa_agent.action_trace import ActionTrace


class SnapshotSession:
    """Read-only session backed by an exported prototype snapshot JSON file."""

    def __init__(self, snapshot_path: str | Path):
        self.snapshot_path = Path(snapshot_path)

    def export_prototype_snapshot(self) -> dict[str, Any]:
        with self.snapshot_path.open("r", encoding="utf-8") as f:
            data = json.load(f)
        if not isinstance(data, dict):
            raise ValueError("Snapshot file must contain a JSON object")
        return data

    def execute_trace(self, trace: ActionTrace, timeout: int = 120):
        raise RuntimeError("SnapshotSession cannot execute traces")

    def close(self) -> None:
        pass
