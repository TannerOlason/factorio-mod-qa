from __future__ import annotations

import random

from mod_qa_agent.action_trace import ActionStep, ActionTrace


DEFAULT_MUTATION_OPERATIONS = (
    "duplicate",
    "delete",
    "swap",
    "insert_wait",
    "append_checkpoint",
    "append_recipe_probe",
    "append_inventory_probe",
    "append_surface_probe",
    "append_entity_probe",
    "append_production_probe",
)


class ActionMutator:
    def __init__(
        self,
        seed: int | None = None,
        operations: tuple[str, ...] | None = None,
    ):
        self.random = random.Random(seed)
        self.operations = operations or DEFAULT_MUTATION_OPERATIONS

    def mutate(
        self,
        trace: ActionTrace,
        mutation_index: int = 1,
        feedback: dict[str, object] | None = None,
    ) -> ActionTrace:
        steps = list(trace.steps)
        if not steps:
            return trace
        operation = self.random.choice(self.operations)
        if operation == "duplicate":
            index = self.random.randrange(len(steps))
            steps.insert(index, steps[index])
        elif operation == "delete" and len(steps) > 1:
            steps.pop(self.random.randrange(len(steps)))
        elif operation == "swap" and len(steps) > 1:
            left = self.random.randrange(len(steps))
            right = self.random.randrange(len(steps))
            steps[left], steps[right] = steps[right], steps[left]
        elif operation == "insert_wait":
            index = self.random.randrange(len(steps) + 1)
            duration = self.random.randint(1, 3)
            steps.insert(
                index,
                ActionStep(
                    kind="wait",
                    code=f"sleep({duration})",
                    metadata={
                        "mutation": operation,
                        "mutation_index": mutation_index,
                    },
                ),
            )
        elif operation == "append_checkpoint":
            steps.append(
                ActionStep(
                    kind="mutation_checkpoint",
                    code=(
                        "print("
                        f"{'QA mutation checkpoint: ' + trace.trace_id!r}"
                        ")"
                    ),
                    metadata={
                        "mutation": operation,
                        "mutation_index": mutation_index,
                    },
                )
            )
        elif operation == "append_recipe_probe":
            recipe_name = _first_step_metadata(steps, "recipe")
            if not recipe_name:
                recipe_name = _first_feedback_value(feedback, "recipes_used")
            steps.append(
                ActionStep(
                    kind="mutation_recipe_probe",
                    code=(
                        f"recipe = get_prototype_recipe({str(recipe_name or '')!r})\n"
                        "print('QA mutation recipe probe:', recipe)"
                    ),
                    metadata={
                        "mutation": operation,
                        "mutation_index": mutation_index,
                        "recipe": recipe_name,
                    },
                )
            )
        elif operation == "append_inventory_probe":
            steps.append(
                ActionStep(
                    kind="mutation_inventory_probe",
                    code=(
                        "inventory = inspect_inventory()\n"
                        "print('QA mutation inventory probe:', inventory)"
                    ),
                    metadata={
                        "mutation": operation,
                        "mutation_index": mutation_index,
                    },
                )
            )
        elif operation == "append_surface_probe":
            surface_name = _surface_from_feedback(feedback)
            steps.append(
                ActionStep(
                    kind="mutation_surface_probe",
                    code=(
                        "print('QA mutation surface probe')\n"
                        "print('player_location:', player_location)"
                    ),
                    metadata={
                        "mutation": operation,
                        "mutation_index": mutation_index,
                        "surface": surface_name,
                    },
                )
            )
        elif operation == "append_entity_probe":
            entity_name = _first_feedback_value(feedback, "entities_built")
            steps.append(
                ActionStep(
                    kind="mutation_entity_probe",
                    code=(
                        f"target_entity = {str(entity_name or '')!r}\n"
                        "entities = get_entities()\n"
                        "if target_entity:\n"
                        "    matching_entities = [\n"
                        "        entity for entity in entities\n"
                        "        if getattr(entity, 'name', None) == target_entity\n"
                        "    ]\n"
                        "else:\n"
                        "    matching_entities = list(entities)[:5]\n"
                        "print(\n"
                        "    'QA mutation entity probe:',\n"
                        "    target_entity or 'sample',\n"
                        "    len(matching_entities),\n"
                        "    matching_entities[:3],\n"
                        ")"
                    ),
                    metadata={
                        "mutation": operation,
                        "mutation_index": mutation_index,
                        "entity": entity_name,
                    },
                )
            )
        elif operation == "append_production_probe":
            material_name = _first_feedback_value(feedback, "items_seen")
            if not material_name:
                material_name = _first_feedback_value(feedback, "fluids_seen")
            recipe_name = _first_step_metadata(steps, "recipe")
            if not recipe_name:
                recipe_name = _first_feedback_value(feedback, "recipes_used")
            steps.append(
                ActionStep(
                    kind="mutation_production_probe",
                    code=(
                        f"target_material = {str(material_name or '')!r}\n"
                        f"target_recipe = {str(recipe_name or '')!r}\n"
                        "production_stats = _get_production_stats()\n"
                        "focused_stats = {}\n"
                        "for bucket, values in production_stats.items():\n"
                        "    if not isinstance(values, dict):\n"
                        "        continue\n"
                        "    if target_material and target_material in values:\n"
                        "        focused_stats[bucket] = values[target_material]\n"
                        "    elif target_recipe and target_recipe in values:\n"
                        "        focused_stats[bucket] = values[target_recipe]\n"
                        "print('QA mutation production probe:', focused_stats)\n"
                        "print('QA mutation production buckets:', production_stats)"
                    ),
                    metadata={
                        "mutation": operation,
                        "mutation_index": mutation_index,
                        "material": material_name,
                        "recipe": recipe_name,
                    },
                )
            )
        else:
            raise ValueError(f"Unknown mutation operation: {operation}")
        steps.append(
            ActionStep(
                kind="wait",
                code="sleep(1)",
                metadata={
                    "mutation": operation,
                    "mutation_index": mutation_index,
                    **(feedback or {}),
                },
            )
        )
        return ActionTrace(
            trace_id=f"{trace.trace_id}-mut-{mutation_index:04d}",
            goal=f"Mutated: {trace.goal}",
            steps=steps,
            seed=trace.seed,
        )


def _first_step_metadata(steps: list[ActionStep], key: str) -> object | None:
    for step in steps:
        value = step.metadata.get(key)
        if value:
            return value
    return None


def _first_feedback_value(
    feedback: dict[str, object] | None,
    key: str,
) -> object | None:
    if not feedback:
        return None
    signature = feedback.get("source_signature")
    if isinstance(signature, dict):
        values = signature.get(key)
        if isinstance(values, (list, tuple)) and values:
            return values[0]
        if values:
            return values
    return None


def _surface_from_feedback(feedback: dict[str, object] | None) -> str | None:
    if not feedback:
        return None
    signature = feedback.get("source_signature")
    if isinstance(signature, dict):
        surface = signature.get("surface")
        if isinstance(surface, str) and surface:
            return surface
    for reason in feedback.get("source_novelty_reasons", []) or []:
        if not isinstance(reason, str):
            continue
        prefix = "surface:"
        if reason.startswith(prefix):
            return reason.removeprefix(prefix).strip()
    return None
