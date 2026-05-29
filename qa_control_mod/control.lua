local function sorted_keys(tbl)
    local keys = {}
    if not tbl then return keys end
    for key, _ in pairs(tbl) do
        table.insert(keys, key)
    end
    table.sort(keys)
    return keys
end

local function safe_get(obj, field)
    if not obj then return nil end
    local ok, value = pcall(function() return obj[field] end)
    if ok then return value end
    return nil
end

local function proto_name(value)
    if not value then return nil end
    if type(value) == "string" then return value end
    local name = safe_get(value, "name")
    if name then return name end
    return tostring(value)
end

local function proto_mod(value)
    if not value then return nil end
    local owner = safe_get(value, "source_mod")
        or safe_get(value, "mod_name")
        or safe_get(value, "mod")
        or safe_get(value, "mod_owner")
    return proto_name(owner)
end

local function serialize_named_set(value)
    local out = {}
    if not value then return out end
    for key, entry in pairs(value) do
        if type(key) == "string" then
            table.insert(out, key)
        else
            local name = proto_name(entry)
            if name then table.insert(out, name) end
        end
    end
    table.sort(out)
    return out
end

local function serialize_ingredients(values)
    local out = {}
    if not values then return out end
    for _, ingredient in pairs(values) do
        table.insert(out, {
            name = ingredient.name,
            type = ingredient.type or "item",
            amount = ingredient.amount or 0
        })
    end
    table.sort(out, function(a, b) return (a.name or "") < (b.name or "") end)
    return out
end

local function serialize_products(values)
    local out = {}
    if not values then return out end
    for _, product in pairs(values) do
        table.insert(out, {
            name = product.name,
            type = product.type or "item",
            amount = product.amount or product.amount_min or 0,
            amount_min = product.amount_min,
            amount_max = product.amount_max,
            probability = product.probability or 1
        })
    end
    table.sort(out, function(a, b) return (a.name or "") < (b.name or "") end)
    return out
end

local function serialize_recipe(recipe)
    return {
        name = recipe.name,
        category = safe_get(recipe, "category"),
        source_mod = proto_mod(recipe),
        enabled = safe_get(recipe, "enabled"),
        hidden = safe_get(recipe, "hidden"),
        energy = safe_get(recipe, "energy"),
        ingredients = serialize_ingredients(safe_get(recipe, "ingredients")),
        products = serialize_products(safe_get(recipe, "products")),
        allow_productivity = safe_get(recipe, "allow_productivity"),
        allow_quality = safe_get(recipe, "allow_quality")
    }
end

local function serialize_item(item)
    return {
        name = item.name,
        type = safe_get(item, "type"),
        source_mod = proto_mod(item),
        stack_size = safe_get(item, "stack_size"),
        fuel_category = safe_get(item, "fuel_category"),
        fuel_value = safe_get(item, "fuel_value"),
        place_result = proto_name(safe_get(item, "place_result")),
        subgroup = proto_name(safe_get(item, "subgroup")),
        order = safe_get(item, "order")
    }
end

local function serialize_fluid(fluid)
    return {
        name = fluid.name,
        source_mod = proto_mod(fluid),
        default_temperature = safe_get(fluid, "default_temperature"),
        max_temperature = safe_get(fluid, "max_temperature"),
        heat_capacity = safe_get(fluid, "heat_capacity"),
        subgroup = proto_name(safe_get(fluid, "subgroup")),
        order = safe_get(fluid, "order")
    }
end

local function serialize_mineable(entity)
    local mineable = safe_get(entity, "mineable_properties")
    if not mineable then return nil end
    return {
        mining_time = mineable.mining_time,
        products = serialize_products(mineable.products),
        required_fluid = mineable.required_fluid,
        fluid_amount = mineable.fluid_amount
    }
end

local function serialize_entity(entity)
    return {
        name = entity.name,
        type = safe_get(entity, "type"),
        source_mod = proto_mod(entity),
        collision_box = safe_get(entity, "collision_box"),
        tile_width = safe_get(entity, "tile_width"),
        tile_height = safe_get(entity, "tile_height"),
        crafting_categories = serialize_named_set(safe_get(entity, "crafting_categories")),
        resource_categories = serialize_named_set(safe_get(entity, "resource_categories")),
        allowed_effects = serialize_named_set(safe_get(entity, "allowed_effects")),
        module_inventory_size = safe_get(entity, "module_inventory_size"),
        mineable_properties = serialize_mineable(entity),
        next_upgrade = proto_name(safe_get(entity, "next_upgrade"))
    }
end

local function serialize_tech_effects(effects)
    local out = {}
    if not effects then return out end
    for _, effect in pairs(effects) do
        local entry = {type = effect.type}
        if effect.recipe then entry.recipe = effect.recipe end
        if effect.modifier then entry.modifier = effect.modifier end
        table.insert(out, entry)
    end
    table.sort(out, function(a, b)
        return (a.type or "") .. (a.recipe or "") < (b.type or "") .. (b.recipe or "")
    end)
    return out
end

local function serialize_technology(tech)
    local ingredients = {}
    for _, ingredient in pairs(safe_get(tech, "research_unit_ingredients") or {}) do
        table.insert(ingredients, {name = ingredient.name, amount = ingredient.amount})
    end
    table.sort(ingredients, function(a, b) return (a.name or "") < (b.name or "") end)
    return {
        name = tech.name,
        source_mod = proto_mod(tech),
        enabled = safe_get(tech, "enabled"),
        hidden = safe_get(tech, "hidden"),
        prerequisites = serialize_named_set(safe_get(tech, "prerequisites")),
        effects = serialize_tech_effects(safe_get(tech, "effects")),
        research_unit_count = safe_get(tech, "research_unit_count"),
        research_unit_energy = safe_get(tech, "research_unit_energy"),
        research_unit_ingredients = ingredients
    }
end

local function serialize_module(module)
    return {
        name = module.name,
        type = safe_get(module, "type"),
        source_mod = proto_mod(module),
        category = safe_get(module, "category"),
        tier = safe_get(module, "tier"),
        limitations = serialize_named_set(safe_get(module, "limitations")),
        limitation_message_key = safe_get(module, "limitation_message_key"),
        effect = safe_get(module, "effect")
    }
end

local function serialize_simple(proto)
    return {
        name = proto.name,
        type = safe_get(proto, "type"),
        source_mod = proto_mod(proto),
        order = safe_get(proto, "order")
    }
end

local function serialize_table(source, serializer)
    local out = {}
    for _, name in ipairs(sorted_keys(source)) do
        local ok, value = pcall(serializer, source[name])
        if ok and value then
            out[name] = value
        end
    end
    return out
end

local function filtered_entity_prototypes(filters)
    local ok, result = pcall(function()
        return game.get_filtered_entity_prototypes(filters)
    end)
    if ok then return result end
    return {}
end

local function filtered_item_prototypes(filters)
    local ok, result = pcall(function()
        return game.get_filtered_item_prototypes(filters)
    end)
    if ok then return result end
    return {}
end

local function encode_json(value)
    if helpers and helpers.table_to_json then
        return helpers.table_to_json(value)
    end
    return game.table_to_json(value)
end

local function decode_json(value)
    if helpers and helpers.json_to_table then
        return helpers.json_to_table(value)
    end
    return game.json_to_table(value)
end

local function serialize_surfaces()
    local out = {}
    for _, surface in pairs(game.surfaces or {}) do
        out[surface.name] = {
            name = surface.name,
            index = surface.index,
            map_gen_seed = surface.map_gen_settings and surface.map_gen_settings.seed or nil
        }
    end
    return out
end

local function export_snapshot()
    local active_mods = safe_get(game, "active_mods") or safe_get(script, "active_mods") or {}
    return encode_json({
        schema_version = 1,
        factorio_version = active_mods.base,
        active_mods = active_mods,
        recipes = serialize_table(prototypes.recipe, serialize_recipe),
        items = serialize_table(prototypes.item, serialize_item),
        fluids = serialize_table(prototypes.fluid, serialize_fluid),
        entities = serialize_table(prototypes.entity, serialize_entity),
        technologies = serialize_table(prototypes.technology, serialize_technology),
        resources = serialize_table(filtered_entity_prototypes({{filter = "type", type = "resource"}}), serialize_entity),
        modules = serialize_table(filtered_item_prototypes({{filter = "type", type = "module"}}), serialize_module),
        crafting_categories = serialize_table(prototypes.recipe_category, serialize_simple),
        resource_categories = serialize_table(prototypes.resource_category, serialize_simple),
        tiles = serialize_table(prototypes.tile, serialize_simple),
        equipment = serialize_table(prototypes.equipment, serialize_simple),
        achievements = serialize_table(prototypes.achievement, serialize_simple),
        surfaces = serialize_surfaces()
    })
end

local function runtime_summary()
    local surfaces = {}
    for _, surface in pairs(game.surfaces or {}) do
        surfaces[surface.name] = {
            entity_count = surface.count_entities_filtered({})
        }
    end
    return encode_json({
        tick = game.tick,
        connected_players = #game.connected_players,
        surfaces = surfaces
    })
end

local function place_entities_batch(json_payload)
    local payload = decode_json(json_payload)
    local surface = game.surfaces[payload.surface or "nauvis"]
    if not surface then error("unknown surface: " .. tostring(payload.surface)) end
    local placed = {}
    for _, spec in pairs(payload.entities or {}) do
        local entity = surface.create_entity({
            name = spec.name,
            position = spec.position or {0, 0},
            force = spec.force or "player",
            direction = spec.direction
        })
        if entity then
            table.insert(placed, {name = entity.name, unit_number = entity.unit_number, position = entity.position})
        end
    end
    return encode_json({placed = placed})
end

local function advance_ticks(ticks)
    local count = tonumber(ticks) or 0
    for _ = 1, count do
        game.tick_paused = false
    end
    return encode_json({requested_ticks = count, tick = game.tick})
end

local function script_event_counts()
    storage.fmqa_event_counts = storage.fmqa_event_counts or {}
    local counts = {}
    for key, value in pairs(storage.fmqa_event_counts) do
        counts[key] = value
    end
    if remote.interfaces["mod_qa_agent"] and remote.interfaces["mod_qa_agent"]["script_event_counts"] then
        local ok, mod_counts = pcall(function()
            return remote.call("mod_qa_agent", "script_event_counts")
        end)
        if ok and type(mod_counts) == "table" then
            for key, value in pairs(mod_counts) do
                counts[key] = value
            end
        end
    end
    return encode_json(counts)
end

local function reset_script_event_counts()
    storage.fmqa_event_counts = {}
    if remote.interfaces["mod_qa_agent"] and remote.interfaces["mod_qa_agent"]["reset_script_event_counts"] then
        pcall(function()
            remote.call("mod_qa_agent", "reset_script_event_counts")
        end)
    end
    return encode_json({ok = true})
end

local function save(name)
    local save_name = name or ("fmqa-" .. tostring(game.tick))
    game.server_save(save_name)
    return encode_json({save = save_name})
end

remote.add_interface("qa_control_mod", {
    export_snapshot = export_snapshot,
    runtime_summary = runtime_summary,
    place_entities_batch = place_entities_batch,
    advance_ticks = advance_ticks,
    script_event_counts = script_event_counts,
    reset_script_event_counts = reset_script_event_counts,
    save = save
})
