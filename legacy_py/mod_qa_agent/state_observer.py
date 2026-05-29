from __future__ import annotations

import json
from typing import Any, Iterable


DEFAULT_FLUID_NAMES = {"water", "steam", "crude-oil"}
RUNTIME_SUMMARY_COMMAND = (
    "/silent-command "
    "local surface = game.surfaces[1] or game.get_surface('nauvis'); "
    "local function count(entity_type) "
    "return #surface.find_entities_filtered{type=entity_type} end; "
    "local function table_count(value) "
    "if type(value) ~= 'table' then return 0 end; "
    "local count = 0; "
    "for _, item in pairs(value) do count = count + 1 end; "
    "return count end; "
    "local script_events = {}; "
    "for _, key in pairs({'mod_qa_events', 'qa_events', "
    "'script_events', 'script_errors'}) do "
    "local event_count = table_count(storage[key]); "
    "if event_count > 0 then script_events[key] = event_count end end; "
    "if remote.interfaces['mod_qa_agent'] and "
    "remote.interfaces['mod_qa_agent']['script_event_counts'] then "
    "local ok, counts = pcall(remote.call, 'mod_qa_agent', 'script_event_counts'); "
    "if ok and type(counts) == 'table' then "
    "for key, value in pairs(counts) do "
    "if type(value) == 'number' and value > 0 then script_events[key] = value end "
    "end end end; "
    "local function enemy_evolution() "
    "local enemy = game.forces.enemy; "
    "if not enemy then return 0 end; "
    "local ok, value = pcall(function() return enemy.evolution_factor end); "
    "if ok and type(value) == 'number' then return value end; "
    "if enemy.get_evolution_factor then "
    "local ok2, value2 = pcall(enemy.get_evolution_factor, enemy, surface); "
    "if ok2 and type(value2) == 'number' then return value2 end "
    "end; "
    "return 0 "
    "end; "
    "local data = {"
    "surface={name=surface.name, index=surface.index}, "
    "power={electric_poles=count('electric-pole'), "
    "accumulators=count('accumulator'), "
    "generators=count('generator'), "
    "solar_panels=count('solar-panel')}, "
    "logistics={roboports=count('roboport'), "
    "logistic_containers=count('logistic-container')}, "
    "circuits={arithmetic_combinators=count('arithmetic-combinator'), "
    "decider_combinators=count('decider-combinator'), "
    "constant_combinators=count('constant-combinator')}, "
    "script_events=script_events, "
    "combat={turrets=count('ammo-turret') + count('electric-turret') + "
    "count('fluid-turret'), spawners=count('unit-spawner'), "
    "units=count('unit'), evolution=enemy_evolution()}"
    "}; "
    "rcon.print(helpers.table_to_json(data))"
)


class StateObserver:
    def __init__(self, instance, known_fluids: Iterable[str] | None = None):
        self.instance = instance
        self.known_fluids = set(known_fluids or DEFAULT_FLUID_NAMES)

    def set_known_fluids(self, fluids: Iterable[str]) -> None:
        self.known_fluids = set(fluids) | DEFAULT_FLUID_NAMES

    def compact_state(self) -> dict[str, Any]:
        namespace = self.instance.namespace
        entities = []
        try:
            entities = [
                getattr(entity, "__dict__", {}) for entity in namespace.get_entities()
            ]
        except Exception:
            entities = []

        research = None
        try:
            research = namespace._save_research_state()
        except Exception:
            research = None

        production_stats = {}
        try:
            production_stats = namespace._get_production_stats()
        except Exception:
            production_stats = {}

        position = None
        try:
            position = namespace.player_location
            position = {"x": position.x, "y": position.y}
        except Exception:
            position = None

        surface = "unknown"
        runtime_summary = {}
        try:
            runtime_summary = self._runtime_summary()
            surface = runtime_summary.get("surface", {}).get("name") or surface
        except Exception:
            try:
                surface = self.instance.rcon_client.send_command(
                    "/silent-command local surface = game.surfaces[1] or "
                    "game.get_surface('nauvis'); rcon.print(surface.name)"
                )
            except Exception:
                pass

        return {
            "inventory": dict(namespace.inspect_inventory().items()),
            "research": research,
            "entities": entities,
            "production_stats": production_stats,
            "position": position,
            "surface": surface,
            "runtime_summary": runtime_summary,
            "power": runtime_summary.get("power", {}),
            "logistics": runtime_summary.get("logistics", {}),
            "circuits": runtime_summary.get("circuits", {}),
            "script_events": runtime_summary.get("script_events", {}),
            "combat": runtime_summary.get("combat", {}),
            "warnings": self.instance.get_warnings(seconds=30),
            "tick": self.instance.get_elapsed_ticks(),
        }

    def _runtime_summary(self) -> dict[str, Any]:
        response = self.instance.rcon_client.send_command(RUNTIME_SUMMARY_COMMAND)
        data = json.loads(response or "{}")
        return data if isinstance(data, dict) else {}

    def signature(self) -> dict[str, Any]:
        state = self.compact_state()
        research = state.get("research")
        techs = []
        if research and getattr(research, "technologies", None):
            techs = sorted(
                name
                for name, tech in research.technologies.items()
                if getattr(tech, "researched", False)
            )

        entities = state.get("entities") or []
        built = sorted(
            {
                entity.get("name")
                for entity in entities
                if entity.get("name") and entity.get("name") != "character"
            }
        )
        recipes = sorted(
            {
                entity.get("recipe", {}).get("name")
                for entity in entities
                if isinstance(entity.get("recipe"), dict)
                and entity.get("recipe", {}).get("name")
            }
        )
        inventory = state.get("inventory") or {}
        stats = state.get("production_stats") or {}
        produced = set(inventory)
        for bucket in ("input", "output", "harvested", "crafted"):
            values = stats.get(bucket, {})
            if isinstance(values, dict):
                produced.update(values)
        power_state = _summary_flags("power", state.get("power"))
        logistic_state = _summary_flags("logistics", state.get("logistics"))
        circuit_state = _summary_flags("circuits", state.get("circuits"))
        script_event_state = _summary_flags(
            "script_events", state.get("script_events")
        )
        combat_state = _summary_flags("combat", state.get("combat"))

        return {
            "techs": techs,
            "items_seen": sorted(produced),
            "recipes_used": recipes,
            "entities_built": built,
            "fluids_seen": sorted(
                item for item in produced if item in self.known_fluids
            ),
            "surface": state.get("surface") or "unknown",
            "power_state": power_state,
            "logistic_state": logistic_state,
            "circuit_state": circuit_state,
            "script_event_state": script_event_state,
            "combat_state": combat_state,
            "milestone_flags": sorted(
                set(state.get("warnings") or [])
                | set(power_state)
                | set(logistic_state)
                | set(circuit_state)
                | set(script_event_state)
                | set(combat_state)
            ),
        }


def _summary_flags(prefix: str, summary: Any) -> list[str]:
    if not isinstance(summary, dict):
        return []
    flags = []
    for key, value in sorted(summary.items()):
        flag_key = f"{prefix}.{key}"
        if isinstance(value, dict):
            flags.extend(_summary_flags(flag_key, value))
        elif isinstance(value, (int, float)) and value:
            flags.append(f"{flag_key}:{value}")
        elif isinstance(value, str) and value:
            flags.append(f"{flag_key}:{value}")
    return flags
