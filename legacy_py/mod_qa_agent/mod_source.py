from __future__ import annotations

import json
import zipfile
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterable


DEFAULT_SOURCE_FILES = {
    "control.lua",
    "data.lua",
    "data-updates.lua",
    "data-final-fixes.lua",
    "settings.lua",
    "settings-updates.lua",
    "settings-final-fixes.lua",
}


@dataclass(frozen=True)
class ModSource:
    path: Path
    name: str
    version: str | None
    info: dict[str, Any]
    files: dict[str, str] = field(default_factory=dict)


def read_mod_source(
    path: str | Path,
    *,
    include_files: Iterable[str] = DEFAULT_SOURCE_FILES,
    max_file_bytes: int = 256_000,
) -> ModSource:
    mod_path = Path(path)
    if mod_path.is_dir():
        return _read_directory_mod(mod_path, set(include_files), max_file_bytes)
    if mod_path.is_file() and mod_path.suffix == ".zip":
        return _read_zip_mod(mod_path, set(include_files), max_file_bytes)
    raise ValueError(f"Unsupported mod source path: {mod_path}")


def list_mod_sources(
    mods_path: str | Path,
    *,
    include_files: Iterable[str] = DEFAULT_SOURCE_FILES,
    max_file_bytes: int = 256_000,
) -> list[ModSource]:
    root = Path(mods_path)
    sources = []
    for child in sorted(root.iterdir()):
        if child.is_dir() or child.suffix == ".zip":
            try:
                sources.append(
                    read_mod_source(
                        child,
                        include_files=include_files,
                        max_file_bytes=max_file_bytes,
                    )
                )
            except (
                KeyError,
                OSError,
                ValueError,
                json.JSONDecodeError,
                zipfile.BadZipFile,
            ):
                continue
    return sources


def summarize_mod_sources(sources: Iterable[ModSource]) -> list[dict[str, Any]]:
    summaries = []
    for source in sources:
        summaries.append(
            {
                "name": source.name,
                "version": source.version,
                "dependencies": _info_list(source.info.get("dependencies")),
                "path": str(source.path),
                "entrypoints": sorted(source.files),
                "entrypoint_bytes": {
                    name: len(content.encode("utf-8"))
                    for name, content in sorted(source.files.items())
                },
            }
        )
    return summaries


def _read_directory_mod(
    path: Path,
    include_files: set[str],
    max_file_bytes: int,
) -> ModSource:
    info = _read_json(path / "info.json")
    files = {}
    for relative_name in include_files:
        file_path = path / relative_name
        if file_path.exists() and file_path.stat().st_size <= max_file_bytes:
            files[relative_name] = file_path.read_text(encoding="utf-8")
    return _source_from_info(path, info, files)


def _read_zip_mod(
    path: Path,
    include_files: set[str],
    max_file_bytes: int,
) -> ModSource:
    with zipfile.ZipFile(path) as archive:
        names = archive.namelist()
        root_prefix = _zip_root_prefix(names)
        info_name = f"{root_prefix}info.json"
        info = json.loads(archive.read(info_name).decode("utf-8"))
        files = {}
        for relative_name in include_files:
            archive_name = f"{root_prefix}{relative_name}"
            if archive_name not in names:
                continue
            info_record = archive.getinfo(archive_name)
            if info_record.file_size <= max_file_bytes:
                files[relative_name] = archive.read(archive_name).decode("utf-8")
    return _source_from_info(path, info, files)


def _read_json(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as f:
        data = json.load(f)
    if not isinstance(data, dict):
        raise ValueError(f"{path} must contain a JSON object")
    return data


def _zip_root_prefix(names: list[str]) -> str:
    for name in names:
        if name.endswith("info.json"):
            return name[: -len("info.json")]
    raise ValueError("Mod zip does not contain info.json")


def _source_from_info(path: Path, info: dict[str, Any], files: dict[str, str]) -> ModSource:
    name = str(info.get("name") or path.stem)
    version = info.get("version")
    return ModSource(
        path=path,
        name=name,
        version=str(version) if version is not None else None,
        info=info,
        files=files,
    )


def _info_list(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, list):
        return [str(item) for item in value]
    return [str(value)]
