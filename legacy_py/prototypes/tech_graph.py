from __future__ import annotations

from dataclasses import dataclass, field

from prototypes.snapshot import PrototypeSnapshot


@dataclass
class TechGraph:
    prerequisites_by_tech: dict[str, set[str]] = field(default_factory=dict)
    unlocks_by_tech: dict[str, set[str]] = field(default_factory=dict)
    techs_by_unlocked_recipe: dict[str, set[str]] = field(default_factory=dict)
    science_packs_by_tech: dict[str, set[str]] = field(default_factory=dict)


def build_tech_graph(snapshot: PrototypeSnapshot) -> TechGraph:
    graph = TechGraph()
    for tech_name, tech in snapshot.technologies.items():
        prerequisites = set(tech.get("prerequisites") or [])
        graph.prerequisites_by_tech[tech_name] = prerequisites

        science_packs = {
            ingredient["name"]
            for ingredient in tech.get("research_unit_ingredients", [])
            if ingredient.get("name")
        }
        graph.science_packs_by_tech[tech_name] = science_packs

        unlocks = set()
        for effect in tech.get("effects", []):
            if effect.get("type") == "unlock-recipe" and effect.get("recipe"):
                unlocks.add(effect["recipe"])
                graph.techs_by_unlocked_recipe.setdefault(effect["recipe"], set()).add(
                    tech_name
                )
        graph.unlocks_by_tech[tech_name] = unlocks
    return graph
