from __future__ import annotations

from dataclasses import dataclass, field
from typing import Iterable

from prototypes.snapshot import PrototypeSnapshot


@dataclass
class RecipeGraph:
    producers_by_material: dict[str, set[str]] = field(default_factory=dict)
    consumers_by_material: dict[str, set[str]] = field(default_factory=dict)
    ingredients_by_recipe: dict[str, set[str]] = field(default_factory=dict)
    products_by_recipe: dict[str, set[str]] = field(default_factory=dict)
    machines_by_category: dict[str, set[str]] = field(default_factory=dict)
    categories_by_machine: dict[str, set[str]] = field(default_factory=dict)
    resource_products: dict[str, set[str]] = field(default_factory=dict)

    def recipes_touching(self, materials: Iterable[str]) -> set[str]:
        recipes = set()
        for material in materials:
            recipes.update(self.producers_by_material.get(material, set()))
            recipes.update(self.consumers_by_material.get(material, set()))
        return recipes


def build_recipe_graph(snapshot: PrototypeSnapshot) -> RecipeGraph:
    graph = RecipeGraph()

    for recipe_name, recipe in snapshot.recipes.items():
        ingredients = {
            ingredient["name"]
            for ingredient in recipe.get("ingredients", [])
            if ingredient.get("name")
        }
        products = {
            product["name"] for product in recipe.get("products", []) if product.get("name")
        }
        graph.ingredients_by_recipe[recipe_name] = ingredients
        graph.products_by_recipe[recipe_name] = products
        for ingredient in ingredients:
            graph.consumers_by_material.setdefault(ingredient, set()).add(recipe_name)
        for product in products:
            graph.producers_by_material.setdefault(product, set()).add(recipe_name)

    for entity_name, entity in snapshot.entities.items():
        categories = set(entity.get("crafting_categories") or [])
        if categories:
            graph.categories_by_machine[entity_name] = categories
        for category in categories:
            graph.machines_by_category.setdefault(category, set()).add(entity_name)

    for resource_name, resource in snapshot.resources.items():
        products = {
            product["name"]
            for product in resource.get("mineable_properties", {}).get("products", [])
            if product.get("name")
        }
        graph.resource_products[resource_name] = products
        for product in products:
            graph.producers_by_material.setdefault(product, set()).add(
                f"resource:{resource_name}"
            )

    return graph
