import json

import pytest

from mod_qa_agent.state_observer import RUNTIME_SUMMARY_COMMAND, StateObserver

pytestmark = pytest.mark.no_factorio


class FakeLocation:
    x = 10
    y = -5


class FakeNamespace:
    player_location = FakeLocation()

    def get_entities(self):
        return []

    def _save_research_state(self):
        return None

    def _get_production_stats(self):
        return {
            "input": {"steam": 10},
            "output": {"molten-iron": 5, "iron-plate": 3},
            "harvested": {},
            "crafted": {},
        }

    def inspect_inventory(self):
        return {"molten-copper": 2, "iron-plate": 1}


class FakeRcon:
    def send_command(self, command):
        if "helpers.table_to_json" in command:
            return json.dumps(
                {
                    "surface": {"name": "vulcanus", "index": 2},
                    "power": {
                        "electric_poles": 2,
                        "accumulators": 0,
                        "networks": {
                            "primary": {
                                "satisfaction": 0.8,
                                "production": 100,
                            },
                        },
                    },
                    "logistics": {
                        "roboports": 1,
                        "logistic_containers": 0,
                        "networks": {"primary": {"robots": 3}},
                    },
                    "circuits": {
                        "arithmetic_combinators": 0,
                        "decider_combinators": 3,
                        "networks": {"red": {"signals": 4}},
                    },
                    "script_events": {"mod_qa_events": 2},
                    "combat": {"turrets": 4, "spawners": 0, "evolution": 0.25},
                }
            )
        return "vulcanus"


class FakeInstance:
    namespace = FakeNamespace()
    rcon_client = FakeRcon()

    def get_warnings(self, seconds=30):
        return []

    def get_elapsed_ticks(self):
        return 123


def test_state_observer_uses_snapshot_fluid_names_in_signature():
    observer = StateObserver(FakeInstance())

    vanilla_signature = observer.signature()
    observer.set_known_fluids(["molten-iron", "molten-copper"])
    modded_signature = observer.signature()

    assert vanilla_signature["fluids_seen"] == ["steam"]
    assert modded_signature["fluids_seen"] == [
        "molten-copper",
        "molten-iron",
        "steam",
    ]
    assert modded_signature["items_seen"] == [
        "iron-plate",
        "molten-copper",
        "molten-iron",
        "steam",
    ]
    assert modded_signature["surface"] == "vulcanus"
    assert modded_signature["power_state"] == [
        "power.electric_poles:2",
        "power.networks.primary.production:100",
        "power.networks.primary.satisfaction:0.8",
    ]
    assert modded_signature["logistic_state"] == [
        "logistics.networks.primary.robots:3",
        "logistics.roboports:1",
    ]
    assert modded_signature["circuit_state"] == [
        "circuits.decider_combinators:3",
        "circuits.networks.red.signals:4",
    ]
    assert modded_signature["script_event_state"] == ["script_events.mod_qa_events:2"]
    assert modded_signature["combat_state"] == [
        "combat.evolution:0.25",
        "combat.turrets:4",
    ]
    assert "power.electric_poles:2" in modded_signature["milestone_flags"]
    assert (
        "power.networks.primary.satisfaction:0.8"
        in modded_signature["milestone_flags"]
    )
    assert "script_events.mod_qa_events:2" in modded_signature["milestone_flags"]


def test_state_observer_falls_back_to_surface_command_when_summary_fails():
    class FallbackRcon:
        def send_command(self, command):
            if "helpers.table_to_json" in command:
                return "not json"
            return "nauvis"

    class FallbackInstance(FakeInstance):
        rcon_client = FallbackRcon()

    signature = StateObserver(FallbackInstance()).signature()

    assert signature["surface"] == "nauvis"
    assert signature["power_state"] == []


def test_runtime_summary_command_works_without_connected_player():
    assert "game.players[1]" not in RUNTIME_SUMMARY_COMMAND
    assert "game.surfaces[1]" in RUNTIME_SUMMARY_COMMAND


def test_runtime_summary_reads_optional_mod_qa_remote_interface():
    assert "remote.call, 'mod_qa_agent', 'script_event_counts'" in RUNTIME_SUMMARY_COMMAND
    assert "reset_script_event_counts" not in RUNTIME_SUMMARY_COMMAND


def test_runtime_summary_guards_factorio_version_specific_combat_fields():
    assert "pcall(function() return enemy.evolution_factor end)" in RUNTIME_SUMMARY_COMMAND
