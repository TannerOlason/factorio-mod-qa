script.on_init(function()
  storage.qa_broken_mod_loaded = true
  storage.mod_qa_events = storage.mod_qa_events or {}
  storage.qa_tick_work = 0
  storage.qa_tick_samples = 0
end)

script.on_event(defines.events.on_tick, function(event)
  if event.tick % 30 ~= 0 then
    return
  end

  storage.mod_qa_events = storage.mod_qa_events or {}
  local machines = game.surfaces.nauvis.find_entities_filtered{name = "qa-ticking-machine"}
  local machine_count = #machines
  if machine_count == 0 then
    return
  end

  -- Intentional fixture bug: work grows quadratically as more machines exist.
  local work = 0
  for _ = 1, machine_count do
    for _ = 1, machine_count do
      work = work + 1
    end
  end

  storage.qa_tick_work = (storage.qa_tick_work or 0) + work
  storage.qa_tick_samples = (storage.qa_tick_samples or 0) + 1
  local limit = math.min(work, 1000)
  for _ = 1, limit do
    storage.mod_qa_events[#storage.mod_qa_events + 1] = {
      tick = event.tick,
      machine_count = machine_count,
      work = work
    }
  end
end)

remote.add_interface("mod_qa_agent", {
  script_event_counts = function()
    return {
      mod_qa_events = storage.mod_qa_events and #storage.mod_qa_events or 0,
      qa_tick_samples = storage.qa_tick_samples or 0,
      qa_tick_work = storage.qa_tick_work or 0
    }
  end,
  reset_script_event_counts = function()
    storage.mod_qa_events = {}
    storage.qa_tick_work = 0
    storage.qa_tick_samples = 0
  end
})
