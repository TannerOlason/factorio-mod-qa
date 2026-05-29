from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from prototypes.recipe_graph import RecipeGraph, build_recipe_graph
from prototypes.snapshot import PrototypeSnapshot
from prototypes.tech_graph import TechGraph, build_tech_graph


@dataclass(frozen=True)
class ValidationIssue:
    code: str
    severity: str
    title: str
    details: dict[str, Any] = field(default_factory=dict)


def validate_snapshot(
    snapshot: PrototypeSnapshot,
    positive_loop_whitelist: set[str] | None = None,
) -> list[ValidationIssue]:
    recipe_graph = build_recipe_graph(snapshot)
    tech_graph = build_tech_graph(snapshot)
    issues: list[ValidationIssue] = []
    issues.extend(validate_recipe_graph(snapshot, recipe_graph))
    issues.extend(validate_tech_graph(snapshot, recipe_graph, tech_graph))
    issues.extend(
        validate_positive_output_loops(
            snapshot, recipe_graph, positive_loop_whitelist or set()
        )
    )
    return issues


def validate_recipe_graph(
    snapshot: PrototypeSnapshot, graph: RecipeGraph
) -> list[ValidationIssue]:
    issues: list[ValidationIssue] = []
    known_materials = snapshot.material_names | snapshot.resource_product_names

    for recipe_name, ingredients in graph.ingredients_by_recipe.items():
        missing = sorted(name for name in ingredients if name not in known_materials)
        if missing:
            issues.append(
                ValidationIssue(
                    code="recipe_missing_ingredient_prototype",
                    severity="error",
                    title=f"Recipe {recipe_name} references unknown ingredients",
                    details={"recipe": recipe_name, "ingredients": missing},
                )
            )

        category = snapshot.recipes.get(recipe_name, {}).get("category")
        if category and category not in graph.machines_by_category:
            issues.append(
                ValidationIssue(
                    code="recipe_missing_crafting_machine",
                    severity="error",
                    title=f"Recipe {recipe_name} has no crafting machine",
                    details={"recipe": recipe_name, "category": category},
                )
            )

    for fluid_name in snapshot.fluids:
        has_source = fluid_name in graph.producers_by_material
        has_sink = fluid_name in graph.consumers_by_material
        if not has_source or not has_sink:
            issues.append(
                ValidationIssue(
                    code="fluid_source_sink_gap",
                    severity="warning",
                    title=f"Fluid {fluid_name} is missing a source or sink",
                    details={
                        "fluid": fluid_name,
                        "has_source": has_source,
                        "has_sink": has_sink,
                    },
                )
            )

    for item_name in snapshot.items:
        has_source = item_name in graph.producers_by_material
        has_sink = item_name in graph.consumers_by_material
        place_result = snapshot.items[item_name].get("place_result")
        if has_source and not has_sink and not place_result:
            issues.append(
                ValidationIssue(
                    code="item_without_use",
                    severity="info",
                    title=f"Item {item_name} has no recipe consumer or place result",
                    details={"item": item_name},
                )
            )
    return issues


def validate_tech_graph(
    snapshot: PrototypeSnapshot, recipe_graph: RecipeGraph, tech_graph: TechGraph
) -> list[ValidationIssue]:
    issues: list[ValidationIssue] = []
    for tech_name, prerequisites in tech_graph.prerequisites_by_tech.items():
        missing_prereqs = sorted(p for p in prerequisites if p not in snapshot.technologies)
        if missing_prereqs:
            issues.append(
                ValidationIssue(
                    code="technology_missing_prerequisite",
                    severity="error",
                    title=f"Technology {tech_name} references missing prerequisites",
                    details={"technology": tech_name, "prerequisites": missing_prereqs},
                )
            )

        missing_science = sorted(
            pack
            for pack in tech_graph.science_packs_by_tech.get(tech_name, set())
            if pack not in recipe_graph.producers_by_material and pack not in snapshot.items
        )
        if missing_science:
            issues.append(
                ValidationIssue(
                    code="technology_unreachable_science_pack",
                    severity="error",
                    title=f"Technology {tech_name} requires unreachable science packs",
                    details={"technology": tech_name, "science_packs": missing_science},
                )
            )

    for tech_name, recipes in tech_graph.unlocks_by_tech.items():
        missing_recipes = sorted(recipe for recipe in recipes if recipe not in snapshot.recipes)
        if missing_recipes:
            issues.append(
                ValidationIssue(
                    code="technology_unlocks_missing_recipe",
                    severity="error",
                    title=f"Technology {tech_name} unlocks missing recipes",
                    details={"technology": tech_name, "recipes": missing_recipes},
                )
            )
    return issues


def validate_positive_output_loops(
    snapshot: PrototypeSnapshot,
    graph: RecipeGraph,
    whitelist: set[str],
) -> list[ValidationIssue]:
    issues: list[ValidationIssue] = []
    for recipe_name, ingredients in graph.ingredients_by_recipe.items():
        if recipe_name in whitelist:
            continue
        products = graph.products_by_recipe.get(recipe_name, set())
        repeated = sorted(ingredients & products)
        if not repeated:
            continue

        recipe = snapshot.recipes.get(recipe_name, {})
        net_positive = []
        for material in repeated:
            input_amount = sum(
                float(i.get("amount") or 0)
                for i in recipe.get("ingredients", [])
                if i.get("name") == material
            )
            output_amount = sum(
                float(p.get("amount") or 0) * float(p.get("probability") or 1)
                for p in recipe.get("products", [])
                if p.get("name") == material
            )
            if output_amount > input_amount:
                net_positive.append(material)

        if net_positive:
            issues.append(
                ValidationIssue(
                    code="positive_output_loop",
                    severity="warning",
                    title=f"Recipe {recipe_name} has positive same-material output",
                    details={"recipe": recipe_name, "materials": net_positive},
                )
            )
    return issues
