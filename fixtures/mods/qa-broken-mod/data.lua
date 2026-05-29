data:extend({
  {
    type = "recipe-category",
    name = "qa-missing-category"
  },
  {
    type = "item",
    name = "qa-ticking-machine",
    icon = "__base__/graphics/icons/assembling-machine-1.png",
    icon_size = 64,
    stack_size = 50,
    place_result = "qa-ticking-machine"
  },
  {
    type = "item",
    name = "qa-unused-item",
    icon = "__base__/graphics/icons/iron-plate.png",
    icon_size = 64,
    stack_size = 100
  },
  {
    type = "recipe",
    name = "qa-ticking-machine",
    category = "crafting",
    enabled = true,
    ingredients = {
      {type = "item", name = "iron-plate", amount = 1}
    },
    results = {
      {type = "item", name = "qa-ticking-machine", amount = 1}
    }
  },
  {
    type = "recipe",
    name = "qa-missing-machine-recipe",
    category = "qa-missing-category",
    enabled = true,
    ingredients = {
      {type = "item", name = "iron-ore", amount = 1}
    },
    results = {
      {type = "item", name = "qa-unused-item", amount = 1}
    }
  },
  {
    type = "recipe",
    name = "qa-positive-loop",
    enabled = true,
    ingredients = {
      {type = "item", name = "qa-unused-item", amount = 1}
    },
    results = {
      {type = "item", name = "qa-unused-item", amount = 2}
    }
  },
  {
    type = "technology",
    name = "qa-bad-technology",
    icon = "__base__/graphics/technology/automation.png",
    icon_size = 256,
    effects = {
      {type = "unlock-recipe", recipe = "qa-positive-loop"}
    },
    unit = {
      count = 1,
      time = 1,
      ingredients = {{"automation-science-pack", 1}}
    }
  }
})

local ticking_machine = table.deepcopy(data.raw["assembling-machine"]["assembling-machine-1"])
ticking_machine.name = "qa-ticking-machine"
ticking_machine.minable = {mining_time = 0.1, result = "qa-ticking-machine"}
ticking_machine.crafting_categories = {"crafting"}
ticking_machine.energy_usage = "1kW"
ticking_machine.energy_source = {type = "void"}
data:extend({ticking_machine})
