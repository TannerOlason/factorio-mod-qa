from __future__ import annotations

import re

from prototypes.snapshot import PrototypeSnapshot
from prototypes.validators import ValidationIssue

from mod_qa_agent.action_mutator import ActionMutator
from mod_qa_agent.action_trace import ActionStep, ActionTrace, TraceExecution
from mod_qa_agent.llm_planner import LLMPlanner


class TraceGenerator:
    def __init__(
        self,
        *,
        goals: int,
        max_traces: int,
        seed: int | None,
        llm_planning: bool = False,
        mod_sources: list[dict[str, object]] | None = None,
        mutations: int = 0,
    ):
        self.goals = goals
        self.max_traces = max_traces
        self.seed = seed
        self.llm_planning = llm_planning
        self.mod_sources = mod_sources or []
        self.mutations = max(0, mutations)

    def generate(
        self, issues: list[ValidationIssue], snapshot: PrototypeSnapshot
    ) -> list[ActionTrace]:
        traces = self._deterministic_traces(issues, snapshot)
        traces.extend(
            LLMPlanner(self.llm_planning, self.mod_sources).propose_traces(
                issues, self.goals, self.seed
            )
        )
        traces = self._with_mutations(traces)
        return traces[: self.max_traces]

    def _with_mutations(self, traces: list[ActionTrace]) -> list[ActionTrace]:
        if not traces or self.mutations <= 0:
            return traces

        mutator = ActionMutator(self.seed)
        mutated = []
        for index in range(self.mutations):
            source = traces[index % len(traces)]
            mutated.append(mutator.mutate(source, mutation_index=index + 1))
        return traces + mutated

    def generate_feedback_mutations(
        self,
        executions: list[TraceExecution],
        count: int,
    ) -> list[ActionTrace]:
        if not executions or count <= 0:
            return []

        ranked = sorted(
            enumerate(executions),
            key=lambda row: self._feedback_rank(row[0], row[1]),
        )
        mutator = ActionMutator(self.seed)
        mutations = []
        for index in range(count):
            execution = ranked[index % len(ranked)][1]
            mutations.append(
                mutator.mutate(
                    execution.trace,
                    mutation_index=index + 1,
                    feedback=self._feedback_metadata(execution),
                )
            )
        return mutations

    @staticmethod
    def _feedback_rank(
        index: int, execution: TraceExecution
    ) -> tuple[int, int, int, int]:
        signature_width = sum(
            len(value) if isinstance(value, (list, tuple, set)) else 1
            for value in execution.signature.values()
            if value
        )
        return (
            0 if execution.error else 1,
            -len(execution.novelty_reasons),
            -signature_width,
            index,
        )

    @staticmethod
    def _feedback_metadata(execution: TraceExecution) -> dict[str, object]:
        interesting_keys = sorted(
            key
            for key, value in execution.signature.items()
            if value not in (None, [], {}, "")
        )
        return {
            "source_trace_id": execution.trace.trace_id,
            "source_error": execution.error,
            "source_novelty_reasons": list(execution.novelty_reasons),
            "source_signature": dict(execution.signature),
            "source_signature_keys": interesting_keys,
        }

    def _deterministic_traces(
        self, issues: list[ValidationIssue], snapshot: PrototypeSnapshot
    ) -> list[ActionTrace]:
        traces = self._script_stress_traces(snapshot)
        for index, issue in enumerate(issues[: self.goals], start=1):
            if len(traces) >= self.goals:
                break
            traces.append(
                ActionTrace(
                    trace_id=f"static-goal-{index:04d}",
                    goal=issue.title,
                    seed=self.seed,
                    steps=[
                        ActionStep(
                            kind="inspect_static_issue",
                            code=(
                                f"print('QA goal: {issue.code}')\n"
                                f"print({issue.details!r})"
                            ),
                            metadata={"issue_code": issue.code},
                        )
                    ],
                )
            )

        if len(traces) < self.goals:
            for recipe_name in sorted(snapshot.recipes)[: self.goals - len(traces)]:
                traces.append(
                    ActionTrace(
                        trace_id=f"recipe-probe-{len(traces) + 1:04d}",
                        goal=f"Probe recipe {recipe_name}",
                        seed=self.seed,
                        steps=[
                            ActionStep(
                                kind="recipe_probe",
                                code=(
                                    f"recipe = get_prototype_recipe({recipe_name!r})\n"
                                    "print(recipe)"
                                ),
                                metadata={"recipe": recipe_name},
                            )
                        ],
                    )
                )
        return traces

    def _script_stress_traces(self, snapshot: PrototypeSnapshot) -> list[ActionTrace]:
        traces = []
        has_non_base_mod = any(name != "base" for name in snapshot.active_mods)
        for entity_name, entity in sorted(snapshot.entities.items()):
            if not isinstance(entity, dict):
                continue
            source_mod = entity.get("source_mod")
            if not _looks_script_backed(entity_name):
                continue
            if source_mod == "base":
                continue
            if not source_mod and not has_non_base_mod:
                continue
            traces.append(
                ActionTrace(
                    trace_id=f"script-stress-{len(traces) + 1:04d}",
                    goal=f"Stress script scaling for {entity_name}",
                    seed=self.seed,
                    steps=[
                        ActionStep(
                            kind="script_stress_probe",
                            code=_script_stress_code(entity_name),
                            metadata={
                                "entity": entity_name,
                                "source_mod": source_mod or "unknown",
                            },
                        )
                    ],
                )
            )
            break
        return traces


def _looks_script_backed(entity_name: str) -> bool:
    normalized = entity_name.replace("_", "-").lower()
    return bool(re.search(r"(^|-)(tick|ticking|script)($|-)", normalized))


def _script_stress_code(entity_name: str) -> str:
    return (
        "entity_name = "
        f"{entity_name!r}\n"
        "lua = \"local surface = game.surfaces[1] or game.get_surface('nauvis'); \" \\\n"
        "      \"local force = game.forces.player; \" \\\n"
        "      \"if remote.interfaces['mod_qa_agent'] and remote.interfaces['mod_qa_agent']['reset_script_event_counts'] then remote.call('mod_qa_agent', 'reset_script_event_counts') end; \" \\\n"
        "      \"for _, entity in pairs(surface.find_entities_filtered{name='\" + entity_name + \"'}) do entity.destroy() end; \" \\\n"
        "      \"for i = 1, 12 do surface.create_entity{name='\" + entity_name + \"', position={i * 3, 0}, force=force, raise_built=true} end\"\n"
        "response = instance.rcon_client.send_command('/silent-command ' + lua)\n"
        "if response:\n"
        "    print(response)\n"
        "sleep(3)\n"
        "print('script stress probe placed ' + entity_name)"
    )
