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

local function ensure_storage()
    storage.fmqa_entities = storage.fmqa_entities or {}
    storage.fmqa_event_counts = storage.fmqa_event_counts or {}
end

local function find_tracked_entity(unit_number)
    ensure_storage()
    local unit = tonumber(unit_number)
    if not unit then return nil end
    local entity = storage.fmqa_entities[unit]
    if entity and entity.valid then return entity end
    for _, surface in pairs(game.surfaces or {}) do
        for _, candidate in pairs(surface.find_entities_filtered({})) do
            if candidate.unit_number == unit then
                storage.fmqa_entities[unit] = candidate
                return candidate
            end
        end
    end
    return nil
end

local function inventory_counts(inventory)
    local counts = {}
    if not inventory or not inventory.valid then return counts end
    local contents = inventory.get_contents()
    for name, count in pairs(contents or {}) do
        if type(count) == "table" and count.name then
            counts[count.name] = (counts[count.name] or 0) + (count.count or 0)
        else
            counts[name] = (counts[name] or 0) + count
        end
    end
    return counts
end

local function first_entity_inventory(entity)
    local candidates = {
        defines.inventory.chest,
        defines.inventory.cargo_wagon,
        defines.inventory.car_trunk,
        defines.inventory.assembling_machine_input,
        defines.inventory.assembling_machine_output,
        defines.inventory.furnace_source,
        defines.inventory.furnace_result,
        defines.inventory.rocket_silo_input,
        defines.inventory.rocket_silo_output
    }
    for _, inventory_id in pairs(candidates) do
        local ok, inventory = pcall(function() return entity.get_inventory(inventory_id) end)
        if ok and inventory and inventory.valid then
            return inventory, inventory_id
        end
    end
    return nil, nil
end

local function snapshot_state(payload)
    local surface = game.surfaces[payload.surface or "nauvis"]
    if not surface then error("unknown surface: " .. tostring(payload.surface)) end
    local entity_counts = {}
    for _, entity in pairs(surface.find_entities_filtered({area = payload.area})) do
        entity_counts[entity.name] = (entity_counts[entity.name] or 0) + 1
    end
    local ground_items = {}
    for _, item_entity in pairs(surface.find_entities_filtered({type = "item-entity", area = payload.area})) do
        if item_entity.stack and item_entity.stack.valid_for_read then
            local name = item_entity.stack.name
            ground_items[name] = (ground_items[name] or 0) + item_entity.stack.count
        end
    end
    return {
        tick = game.tick,
        surface = surface.name,
        entity_counts = entity_counts,
        ground_items = ground_items
    }
end

local function create_surface(payload)
    local name = payload.name
    if not name or name == "" then error("surface name is required") end
    local existing = game.surfaces[name]
    if existing then
        return {name = existing.name, index = existing.index, created = false}
    end
    local surface = game.create_surface(name, payload.map_gen_settings or {})
    surface.request_to_generate_chunks(payload.position or {0, 0}, payload.chunk_radius or 2)
    surface.force_generate_chunk_requests()
    return {name = surface.name, index = surface.index, created = true}
end

local function delete_surface(payload)
    local name = payload.name
    if not name or name == "" then error("surface name is required") end
    if name == "nauvis" then error("refusing to delete nauvis") end
    local surface = game.surfaces[name]
    if not surface then return {deleted = false, name = name} end
    game.delete_surface(surface)
    return {deleted = true, name = name}
end

local function find_buildable_position(payload)
    local surface = game.surfaces[payload.surface or "nauvis"]
    if not surface then error("unknown surface: " .. tostring(payload.surface)) end
    local entity = payload.entity or "steel-chest"
    local position = payload.position or {0, 0}
    local radius = payload.radius or 64
    local precision = payload.precision or 1
    local found = surface.find_non_colliding_position(entity, position, radius, precision)
    return {
        surface = surface.name,
        entity = entity,
        position = found
    }
end

local function place_entity(payload)
    ensure_storage()
    local surface = game.surfaces[payload.surface or "nauvis"]
    if not surface then error("unknown surface: " .. tostring(payload.surface)) end
    local entity = surface.create_entity({
        name = payload.name,
        position = payload.position or {0, 0},
        force = payload.force or "player",
        direction = payload.direction,
        raise_built = false
    })
    if not entity then error("failed to create entity: " .. tostring(payload.name)) end
    if entity.unit_number then
        storage.fmqa_entities[entity.unit_number] = entity
    end
    return {name = entity.name, unit_number = entity.unit_number, position = entity.position}
end

local function insert_items(payload)
    local entity = find_tracked_entity(payload.unit_number)
    if not entity then error("unknown entity unit_number: " .. tostring(payload.unit_number)) end
    local inventory = first_entity_inventory(entity)
    if not inventory then error("entity has no supported inventory: " .. entity.name) end
    local inserted = {}
    for name, count in pairs(payload.items or {}) do
        inserted[name] = inventory.insert({name = name, count = count})
    end
    return {unit_number = entity.unit_number, inserted = inserted, inventory = inventory_counts(inventory)}
end

local function read_entity_inventory(payload)
    local entity = find_tracked_entity(payload.unit_number)
    if not entity then error("unknown entity unit_number: " .. tostring(payload.unit_number)) end
    local inventory = first_entity_inventory(entity)
    return {
        unit_number = entity.unit_number,
        name = entity.name,
        inventory = inventory_counts(inventory)
    }
end

local function mine_entity_to_inventory(payload)
    local entity = find_tracked_entity(payload.unit_number)
    if not entity then error("unknown entity unit_number: " .. tostring(payload.unit_number)) end
    local buffer = game.create_inventory(payload.buffer_size or 100)
    local mined = entity.mine({
        inventory = buffer,
        force = payload.force_mine == true,
        raise_destroyed = false,
        ignore_minable = false
    })
    local counts = inventory_counts(buffer)
    buffer.destroy()
    return {mined = mined, inventory = counts}
end

local function remove_entity(payload)
    local entity = find_tracked_entity(payload.unit_number)
    if not entity then return {removed = false} end
    local unit = entity.unit_number
    entity.destroy({raise_destroy = false})
    if unit then storage.fmqa_entities[unit] = nil end
    return {removed = true, unit_number = unit}
end

local blueprint_config_fields = {
    "ammo_inventory",
    "bar",
    "connections",
    "control_behavior",
    "filters",
    "infinity_settings",
    "inventory",
    "items",
    "parameters",
    "recipe",
    "request_filters",
    "station",
    "tags",
    "trunk_inventory"
}

local function first_blueprint(payload)
    local root = payload.blueprint or payload
    if root.blueprint then return root.blueprint end
    local book = root.blueprint_book
    if book and book.blueprints then
        for _, entry in pairs(book.blueprints) do
            if entry.blueprint then return entry.blueprint end
        end
    end
    error("blueprint payload must contain blueprint or blueprint_book")
end

local function xy(position)
    position = position or {}
    return {
        x = position.x or position[1] or 0,
        y = position.y or position[2] or 0
    }
end

local function offset_position(base, relative)
    base = xy(base)
    relative = xy(relative)
    return {x = base.x + relative.x, y = base.y + relative.y}
end

local function recipe_name(entity)
    if not (entity and entity.valid and entity.get_recipe) then return nil end
    local ok, recipe = pcall(function() return entity.get_recipe() end)
    if not ok or not recipe then return nil end
    return proto_name(recipe)
end

local function module_inventory(entity)
    if not (entity and entity.valid and entity.get_module_inventory) then return nil end
    local ok, inventory = pcall(function() return entity.get_module_inventory() end)
    if ok and inventory and inventory.valid then return inventory end
    return nil
end

local function tracked_entity_snapshot(entity, blueprint_recipe)
    if not (entity and entity.valid) then return nil end
    local module_inv = module_inventory(entity)
    local inventory = first_entity_inventory(entity)
    return {
        name = entity.name,
        unit_number = entity.unit_number,
        position = entity.position,
        blueprint_recipe = blueprint_recipe,
        actual_recipe = recipe_name(entity),
        recipe_locked = safe_get(entity, "recipe_locked"),
        inventory = inventory_counts(inventory),
        module_inventory = inventory_counts(module_inv)
    }
end

local function configured_fields(spec)
    local fields = {}
    for _, field in pairs(blueprint_config_fields) do
        if spec[field] ~= nil then fields[#fields + 1] = field end
    end
    table.sort(fields)
    return fields
end

local function nil_if_empty(list)
    if list and #list > 0 then return list end
    return nil
end

local function insert_blueprint_items(entity, items)
    if not items then return {} end
    local inserted = {}
    local module_inv = module_inventory(entity)
    local inventory = first_entity_inventory(entity)
    for name, count in pairs(items) do
        local remaining = count
        if module_inv then
            local n = module_inv.insert({name = name, count = remaining})
            inserted[name] = (inserted[name] or 0) + n
            remaining = remaining - n
        end
        if remaining > 0 and inventory then
            local n = inventory.insert({name = name, count = remaining})
            inserted[name] = (inserted[name] or 0) + n
        end
    end
    return inserted
end

local function apply_blueprint_entity_config(entity, spec)
    local applied = {}
    if spec.recipe and entity and entity.valid and entity.set_recipe then
        local ok, err = pcall(function() entity.set_recipe(spec.recipe) end)
        applied.recipe = {ok = ok, error = ok and nil or tostring(err)}
    end
    if spec.items and entity and entity.valid then
        applied.items = insert_blueprint_items(entity, spec.items)
    end
    return applied
end

local function create_blueprint_entity(surface, spec, payload, mode)
    local position = offset_position(payload.position, spec.position)
    local force = payload.force or "player"
    if mode == "ghost" then
        return surface.create_entity({
            name = "entity-ghost",
            inner_name = spec.name,
            position = position,
            force = force,
            direction = spec.direction,
            expires = false
        })
    end
    return surface.create_entity({
        name = spec.name,
        position = position,
        force = force,
        direction = spec.direction,
        raise_built = payload.raise_built ~= false,
        create_build_effect_smoke = false
    })
end

local function blueprint_smoke(payload)
    ensure_storage()
    local surface = game.surfaces[payload.surface or "nauvis"]
    if not surface then error("unknown surface: " .. tostring(payload.surface)) end
    local mode = payload.mode or "instant"
    if mode ~= "ghost" and mode ~= "instant" then error("unknown blueprint smoke mode: " .. tostring(mode)) end
    local blueprint = first_blueprint(payload)
    local created = {}
    local missing = {}
    local configured = {}
    local entities = blueprint.entities or {}

    for _, spec in pairs(entities) do
        local fields = configured_fields(spec)
        if #fields > 0 then
            configured[#configured + 1] = {
                entity_number = spec.entity_number,
                name = spec.name,
                fields = fields,
                recipe = spec.recipe
            }
        end

        if not spec.name then
            missing[#missing + 1] = {name = "<missing>", reason = "entity name is missing"}
        else
            local ok, entity_or_error = pcall(function()
                return create_blueprint_entity(surface, spec, payload, mode)
            end)
            local entity = ok and entity_or_error or nil
            if not (entity and entity.valid) then
                missing[#missing + 1] = {name = spec.name, reason = ok and "create_entity returned nil" or tostring(entity_or_error)}
            else
                if mode == "instant" then
                    apply_blueprint_entity_config(entity, spec)
                end
                if entity.unit_number then
                    storage.fmqa_entities[entity.unit_number] = entity
                end
                created[#created + 1] = tracked_entity_snapshot(entity, spec.recipe)
            end
        end
    end

    return {
        mode = mode,
        surface = surface.name,
        expected_entities = #entities,
        created_count = #created,
        created = nil_if_empty(created),
        missing = nil_if_empty(missing),
        configured = nil_if_empty(configured)
    }
end

local function read_tracked_entities(payload)
    local entities = {}
    for _, unit_number in pairs(payload.unit_numbers or {}) do
        local entity = find_tracked_entity(unit_number)
        if entity then
            entities[#entities + 1] = tracked_entity_snapshot(entity, nil)
        end
    end
    return {entities = nil_if_empty(entities)}
end

local function place_entities_batch(json_payload)
    local payload = decode_json(json_payload)
    local surface = game.surfaces[payload.surface or "nauvis"]
    if not surface then error("unknown surface: " .. tostring(payload.surface)) end
    local placed = {}
    ensure_storage()
    for _, spec in pairs(payload.entities or {}) do
        local entity = surface.create_entity({
            name = spec.name,
            position = spec.position or {0, 0},
            force = spec.force or "player",
            direction = spec.direction
        })
        if entity then
            if entity.unit_number then
                storage.fmqa_entities[entity.unit_number] = entity
            end
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
    ensure_storage()
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
    ensure_storage()
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

local function dispatch(command, json_payload)
    local handlers = {
        runtime_summary = function(payload)
            return decode_json(runtime_summary())
        end,
        snapshot_state = function(payload)
            return snapshot_state(payload or {})
        end,
        create_surface = function(payload)
            return create_surface(payload or {})
        end,
        delete_surface = function(payload)
            return delete_surface(payload or {})
        end,
        find_buildable_position = function(payload)
            return find_buildable_position(payload or {})
        end,
        place_entity = function(payload)
            return place_entity(payload or {})
        end,
        place_entities_batch = function(payload)
            return decode_json(place_entities_batch(encode_json(payload or {})))
        end,
        insert_items = function(payload)
            return insert_items(payload or {})
        end,
        read_entity_inventory = function(payload)
            return read_entity_inventory(payload or {})
        end,
        mine_entity_to_inventory = function(payload)
            return mine_entity_to_inventory(payload or {})
        end,
        remove_entity = function(payload)
            return remove_entity(payload or {})
        end,
        blueprint_smoke = function(payload)
            return blueprint_smoke(payload or {})
        end,
        read_tracked_entities = function(payload)
            return read_tracked_entities(payload or {})
        end,
        advance_ticks = function(payload)
            return decode_json(advance_ticks(payload and payload.ticks or 0))
        end,
        script_event_counts = function(payload)
            return decode_json(script_event_counts())
        end,
        reset_script_event_counts = function(payload)
            return decode_json(reset_script_event_counts())
        end,
        save = function(payload)
            return decode_json(save(payload and payload.name or nil))
        end
    }

    local handler = handlers[command]
    if not handler then
        return encode_json({ok = false, error = "unknown qa_control_mod command: " .. tostring(command)})
    end

    local ok_payload, payload = pcall(function()
        if not json_payload or json_payload == "" then return {} end
        return decode_json(json_payload)
    end)
    if not ok_payload then
        return encode_json({ok = false, error = "invalid JSON payload: " .. tostring(payload)})
    end

    local ok, result = pcall(function()
        return handler(payload or {})
    end)
    if not ok then
        return encode_json({ok = false, error = tostring(result)})
    end
    return encode_json({ok = true, result = result or {}})
end

remote.add_interface("qa_control_mod", {
    export_snapshot = export_snapshot,
    runtime_summary = runtime_summary,
    place_entities_batch = place_entities_batch,
    advance_ticks = advance_ticks,
    script_event_counts = script_event_counts,
    reset_script_event_counts = reset_script_event_counts,
    save = save,
    dispatch = dispatch
})
