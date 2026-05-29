from __future__ import annotations

import json
from dataclasses import asdict, dataclass, field
from typing import Any


@dataclass(frozen=True)
class ActionStep:
    kind: str
    code: str
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class ActionTrace:
    trace_id: str
    goal: str
    steps: list[ActionStep]
    seed: int | None = None

    def to_jsonl(self) -> str:
        lines = []
        for index, step in enumerate(self.steps):
            row = {
                "trace_id": self.trace_id,
                "goal": self.goal,
                "step_index": index,
                "seed": self.seed,
                **asdict(step),
            }
            lines.append(json.dumps(row, sort_keys=True))
        return "\n".join(lines) + ("\n" if lines else "")

    @classmethod
    def from_jsonl(cls, payload: str) -> "ActionTrace":
        rows = [
            json.loads(line)
            for line in payload.splitlines()
            if line.strip()
        ]
        if not rows:
            raise ValueError("Trace JSONL payload is empty")

        first = rows[0]
        trace_id = first["trace_id"]
        goal = first["goal"]
        seed = first.get("seed")
        steps = []
        for row in sorted(rows, key=lambda item: item["step_index"]):
            if row["trace_id"] != trace_id or row["goal"] != goal:
                raise ValueError("Trace JSONL rows do not describe one trace")
            steps.append(
                ActionStep(
                    kind=row["kind"],
                    code=row["code"],
                    metadata=row.get("metadata") or {},
                )
            )
        return cls(trace_id=trace_id, goal=goal, steps=steps, seed=seed)

    def code(self) -> str:
        return "\n".join(step.code for step in self.steps)


@dataclass(frozen=True)
class ExecutionError:
    kind: str
    message: str
    source: str
    details: dict[str, Any] = field(default_factory=dict)


@dataclass
class TraceExecution:
    trace: ActionTrace
    response: str
    error: bool
    duration: float = 0.0
    game_state_raw: str | None = None
    signature: dict[str, Any] = field(default_factory=dict)
    native_save_path: str | None = None
    error_details: ExecutionError | None = None
    novelty_reasons: list[str] = field(default_factory=list)
