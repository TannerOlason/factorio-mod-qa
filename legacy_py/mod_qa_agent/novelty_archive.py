from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable

from fle.commons.models.game_state import GameState

from mod_qa_agent.action_trace import ActionTrace, TraceExecution


@dataclass(frozen=True)
class ArchivedState:
    state_path: Path
    signature_path: Path
    trace_path: Path
    state_raw: str
    signature_payload: dict[str, Any]
    trace: ActionTrace

    @property
    def native_save_path(self) -> str | None:
        value = self.signature_payload.get("native_save_path")
        return str(value) if value else None

    @property
    def native_save_name(self) -> str | None:
        path = self.native_save_path
        if not path:
            return None
        return Path(path).stem

    @property
    def novelty_reasons(self) -> list[str]:
        reasons = self.signature_payload.get("novelty_reasons") or []
        return [str(reason) for reason in reasons]

    def game_state(self) -> GameState:
        return GameState.parse_raw(self.state_raw)

    def restore_to(self, target) -> None:
        instance = getattr(target, "instance", target)
        self.game_state().to_instance(instance)

    def reload_native_save(self, target) -> str:
        save_name = self.native_save_name
        if not save_name:
            raise ValueError("Archived state does not reference a native save")
        instance = getattr(target, "instance", target)
        response = instance.rcon_client.send_command(f"/load {save_name}")
        if response and "error" in response.lower():
            raise RuntimeError(response)
        return response or ""


class NoveltyArchive:
    def __init__(
        self,
        archive_dir: str | Path,
        native_save_callback: Callable[[str], str | None] | None = None,
    ):
        self.archive_dir = Path(archive_dir)
        self.archive_dir.mkdir(parents=True, exist_ok=True)
        self.native_save_callback = native_save_callback
        self._counter = 0
        self._seen: dict[str, set[str]] = {
            "techs": set(),
            "items_seen": set(),
            "recipes_used": set(),
            "entities_built": set(),
            "fluids_seen": set(),
            "surface": set(),
            "power_state": set(),
            "logistic_state": set(),
            "circuit_state": set(),
            "combat_state": set(),
            "milestone_flags": set(),
        }

    def load(self, index: int | str | Path) -> ArchivedState:
        if isinstance(index, Path):
            state_path = index
            suffix = self._suffix_from_state_path(state_path)
            archive_dir = state_path.parent
        else:
            suffix = f"{int(index):06d}" if isinstance(index, int) else str(index)
            if suffix.startswith("state_"):
                suffix = suffix.removeprefix("state_")
            suffix = suffix.removesuffix(".json")
            archive_dir = self.archive_dir
            state_path = archive_dir / f"state_{suffix}.json"

        signature_path = archive_dir / f"signature_{suffix}.json"
        trace_path = archive_dir / f"trace_{suffix}.jsonl"

        state_raw = state_path.read_text(encoding="utf-8")
        signature_payload = json.loads(signature_path.read_text(encoding="utf-8"))
        trace = ActionTrace.from_jsonl(trace_path.read_text(encoding="utf-8"))
        return ArchivedState(
            state_path=state_path,
            signature_path=signature_path,
            trace_path=trace_path,
            state_raw=state_raw,
            signature_payload=signature_payload,
            trace=trace,
        )

    @staticmethod
    def _suffix_from_state_path(path: Path) -> str:
        name = path.name
        if not name.startswith("state_") or not name.endswith(".json"):
            raise ValueError("Archive state path must look like state_<suffix>.json")
        return name.removeprefix("state_").removesuffix(".json")

    def novelty_reasons(self, signature: dict[str, Any]) -> list[str]:
        reasons = []
        for key, seen in self._seen.items():
            values = signature.get(key, [])
            if isinstance(values, str):
                values = [values]
            new_values = sorted(str(value) for value in values if str(value) not in seen)
            if new_values:
                reasons.append(f"{key}: {', '.join(new_values[:8])}")
        return reasons

    def archive_if_interesting(self, execution: TraceExecution) -> Path | None:
        reasons = self.novelty_reasons(execution.signature)
        execution.novelty_reasons = reasons
        if not reasons and not execution.error:
            return None

        self._counter += 1
        suffix = f"{self._counter:06d}"
        state_path = self.archive_dir / f"state_{suffix}.json"
        signature_path = self.archive_dir / f"signature_{suffix}.json"
        trace_path = self.archive_dir / f"trace_{suffix}.jsonl"

        state_path.write_text(execution.game_state_raw or "{}", encoding="utf-8")
        native_save_path = None
        if self.native_save_callback:
            native_save_path = self.native_save_callback(f"mod_qa_state_{suffix}")
            execution.native_save_path = native_save_path
        signature_payload = {
            "trace_id": execution.trace.trace_id,
            "goal": execution.trace.goal,
            "novelty_reasons": reasons,
            "error": execution.error,
            "native_save_path": native_save_path,
            "signature": execution.signature,
        }
        signature_path.write_text(
            json.dumps(signature_payload, indent=2, sort_keys=True),
            encoding="utf-8",
        )
        trace_path.write_text(execution.trace.to_jsonl(), encoding="utf-8")

        for key, seen in self._seen.items():
            values = execution.signature.get(key, [])
            if isinstance(values, str):
                values = [values]
            seen.update(str(value) for value in values)

        return state_path
