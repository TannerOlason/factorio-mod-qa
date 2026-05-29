from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Mapping


def _dict_section(data: Mapping[str, Any], key: str) -> dict[str, Any]:
    value = data.get(key, {})
    return value if isinstance(value, dict) else {}


@dataclass(frozen=True)
class PrototypeSnapshot:
    schema_version: int
    factorio_version: str | None
    active_mods: dict[str, str]
    recipes: dict[str, dict[str, Any]] = field(default_factory=dict)
    items: dict[str, dict[str, Any]] = field(default_factory=dict)
    fluids: dict[str, dict[str, Any]] = field(default_factory=dict)
    entities: dict[str, dict[str, Any]] = field(default_factory=dict)
    technologies: dict[str, dict[str, Any]] = field(default_factory=dict)
    resources: dict[str, dict[str, Any]] = field(default_factory=dict)
    modules: dict[str, dict[str, Any]] = field(default_factory=dict)
    crafting_categories: dict[str, dict[str, Any]] = field(default_factory=dict)
    resource_categories: dict[str, dict[str, Any]] = field(default_factory=dict)
    tiles: dict[str, dict[str, Any]] = field(default_factory=dict)
    equipment: dict[str, dict[str, Any]] = field(default_factory=dict)
    achievements: dict[str, dict[str, Any]] = field(default_factory=dict)
    surfaces: dict[str, dict[str, Any]] = field(default_factory=dict)
    prototype_mods: dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Mapping[str, Any]) -> "PrototypeSnapshot":
        return cls(
            schema_version=int(data.get("schema_version", 0) or 0),
            factorio_version=data.get("factorio_version"),
            active_mods=dict(_dict_section(data, "active_mods")),
            recipes=_dict_section(data, "recipes"),
            items=_dict_section(data, "items"),
            fluids=_dict_section(data, "fluids"),
            entities=_dict_section(data, "entities"),
            technologies=_dict_section(data, "technologies"),
            resources=_dict_section(data, "resources"),
            modules=_dict_section(data, "modules"),
            crafting_categories=_dict_section(data, "crafting_categories"),
            resource_categories=_dict_section(data, "resource_categories"),
            tiles=_dict_section(data, "tiles"),
            equipment=_dict_section(data, "equipment"),
            achievements=_dict_section(data, "achievements"),
            surfaces=_dict_section(data, "surfaces"),
            prototype_mods=_dict_section(data, "prototype_mods"),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "schema_version": self.schema_version,
            "factorio_version": self.factorio_version,
            "active_mods": self.active_mods,
            "recipes": self.recipes,
            "items": self.items,
            "fluids": self.fluids,
            "entities": self.entities,
            "technologies": self.technologies,
            "resources": self.resources,
            "modules": self.modules,
            "crafting_categories": self.crafting_categories,
            "resource_categories": self.resource_categories,
            "tiles": self.tiles,
            "equipment": self.equipment,
            "achievements": self.achievements,
            "surfaces": self.surfaces,
            "prototype_mods": self.prototype_mods,
        }

    @property
    def material_names(self) -> set[str]:
        return set(self.items) | set(self.fluids)

    @property
    def resource_product_names(self) -> set[str]:
        names = set()
        for resource in self.resources.values():
            for product in resource.get("mineable_properties", {}).get("products", []):
                if product.get("name"):
                    names.add(product["name"])
        return names


def load_snapshot(path: str | Path) -> PrototypeSnapshot:
    with Path(path).open("r", encoding="utf-8") as f:
        return PrototypeSnapshot.from_dict(json.load(f))
