import json
import zipfile
from pathlib import Path

import pytest

from mod_qa_agent.mod_source import (
    list_mod_sources,
    read_mod_source,
    summarize_mod_sources,
)

pytestmark = pytest.mark.no_factorio

FIXTURE_MODS_DIR = Path(__file__).parents[3] / "fixtures" / "mods"


def write_mod_dir(path, name="debug-mod", version="0.1.0", dependencies=None):
    path.mkdir()
    (path / "info.json").write_text(
        json.dumps(
            {
                "name": name,
                "version": version,
                "dependencies": dependencies or [],
            }
        ),
        encoding="utf-8",
    )
    (path / "control.lua").write_text("script.on_init(function() end)", encoding="utf-8")
    (path / "thumbnail.png").write_bytes(b"not read")


def test_read_mod_source_from_directory(tmp_path):
    mod_path = tmp_path / "debug-mod"
    write_mod_dir(mod_path)

    source = read_mod_source(mod_path)

    assert source.name == "debug-mod"
    assert source.version == "0.1.0"
    assert source.info["name"] == "debug-mod"
    assert source.info["dependencies"] == []
    assert source.files == {"control.lua": "script.on_init(function() end)"}


def test_read_mod_source_from_zip(tmp_path):
    zip_path = tmp_path / "debug-mod_0.1.0.zip"
    with zipfile.ZipFile(zip_path, "w") as archive:
        archive.writestr(
            "debug-mod_0.1.0/info.json",
            json.dumps({"name": "debug-mod", "version": "0.1.0"}),
        )
        archive.writestr("debug-mod_0.1.0/data.lua", "data:extend({})")

    source = read_mod_source(zip_path)

    assert source.name == "debug-mod"
    assert source.files == {"data.lua": "data:extend({})"}


def test_list_mod_sources_reads_directories_and_zips(tmp_path):
    write_mod_dir(tmp_path / "alpha", name="alpha")
    with zipfile.ZipFile(tmp_path / "beta_0.1.0.zip", "w") as archive:
        archive.writestr(
            "beta_0.1.0/info.json",
            json.dumps({"name": "beta", "version": "0.1.0"}),
        )
    (tmp_path / "readme.txt").write_text("ignored", encoding="utf-8")

    sources = list_mod_sources(tmp_path)

    assert [source.name for source in sources] == ["alpha", "beta"]


def test_list_mod_sources_skips_non_mod_directories_and_bad_zips(tmp_path):
    write_mod_dir(tmp_path / "alpha", name="alpha")
    (tmp_path / "notes").mkdir()
    (tmp_path / "notes" / "readme.txt").write_text("not a mod", encoding="utf-8")
    (tmp_path / "broken.zip").write_text("not a zip", encoding="utf-8")

    sources = list_mod_sources(tmp_path)

    assert [source.name for source in sources] == ["alpha"]


def test_summarize_mod_sources_omits_source_contents(tmp_path):
    mod_path = tmp_path / "debug-mod"
    write_mod_dir(mod_path, dependencies=["base >= 2.0.0", "? space-age"])

    summary = summarize_mod_sources([read_mod_source(mod_path)])

    assert summary == [
        {
            "name": "debug-mod",
            "version": "0.1.0",
            "dependencies": ["base >= 2.0.0", "? space-age"],
            "path": str(mod_path),
            "entrypoints": ["control.lua"],
            "entrypoint_bytes": {"control.lua": len("script.on_init(function() end)")},
        }
    ]


def test_read_intentionally_broken_fixture_mod_source():
    source = read_mod_source(FIXTURE_MODS_DIR / "qa-broken-mod")

    assert source.name == "qa-broken-mod"
    assert source.version == "0.1.0"
    assert sorted(source.files) == ["control.lua", "data.lua"]
    assert "qa-missing-machine-recipe" in source.files["data.lua"]
