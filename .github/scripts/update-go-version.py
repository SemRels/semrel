#!/usr/bin/env python3
"""Add the latest stable Go release line to the CI test matrix."""

from __future__ import annotations

import json
import re
import sys
import urllib.request
from pathlib import Path

SOURCE = "https://go.dev/dl/?mode=json&include=all"
VERSION = re.compile(r"^go(?P<major>[0-9]+)\.(?P<minor>[0-9]+)(?:\.(?P<patch>[0-9]+))?$")
MATRIX = re.compile(r"(?m)^(?P<indent>\s*)go:\s*\[(?P<values>[^\]]*)\]")


def stable_version() -> tuple[str, tuple[int, int, int]]:
    request = urllib.request.Request(SOURCE, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            payload = json.load(response)
    except Exception as error:
        raise RuntimeError(f"unable to read Go release data: {error}") from error

    if not isinstance(payload, list):
        raise RuntimeError("Go release data is not a JSON array")

    releases: list[tuple[str, tuple[int, int, int]]] = []
    for release in payload:
        if not isinstance(release, dict) or release.get("stable") is not True:
            continue
        version = release.get("version")
        if not isinstance(version, str):
            raise RuntimeError("stable Go release has no string version")
        # The API includes the historical "go1" family marker.
        if version == "go1":
            continue
        match = VERSION.fullmatch(version)
        if not match:
            raise RuntimeError(f"malformed stable Go version: {version!r}")
        releases.append(
            (
                version,
                (
                    int(match["major"]),
                    int(match["minor"]),
                    int(match["patch"] or 0),
                ),
            )
        )

    if not releases:
        raise RuntimeError("Go release data contains no stable releases")
    return max(releases, key=lambda release: release[1])


def matrix_versions(path: Path) -> set[tuple[int, int]]:
    versions: set[tuple[int, int]] = set()
    for match in MATRIX.finditer(path.read_text(encoding="utf-8")):
        for value in match["values"].split(","):
            value = value.strip().strip("\"'")
            if not value:
                continue
            value_match = re.fullmatch(
                r"(?P<major>[0-9]+)\.(?P<minor>[0-9]+)(?:\.[0-9]+)?(?:[-+].*)?",
                value,
            )
            if value_match:
                versions.add((int(value_match["major"]), int(value_match["minor"])))
    return versions


def update_matrix(path: Path, version: str, release_line: tuple[int, int]) -> bool:
    content = path.read_text(encoding="utf-8")
    match = MATRIX.search(content)
    if not match:
        raise RuntimeError(f"no Go test matrix found in {path}")
    if release_line in matrix_versions(path):
        return False

    values = match["values"].rstrip()
    replacement = f"{match['indent']}go: [{values}, \"{version.removeprefix('go')}\"]"
    path.write_text(content[: match.start()] + replacement + content[match.end() :], encoding="utf-8")
    return True


def main() -> int:
    target = Path(".github/workflows/ci.yaml")
    workflow_files = sorted(Path(".github/workflows").glob("*.y*ml"))
    if not target.exists() or not workflow_files:
        raise RuntimeError("expected CI workflow files are missing")

    release, release_tuple = stable_version()
    current = sorted(
        {
            version
            for workflow in workflow_files
            for version in matrix_versions(workflow)
        }
    )
    changed = update_matrix(target, release, release_tuple[:2])
    print(f"latest={release}")
    print(f"matrix_versions={','.join(f'{major}.{minor}' for major, minor in current)}")
    print(f"updated={'true' if changed else 'false'}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except RuntimeError as error:
        print(f"error: {error}", file=sys.stderr)
        raise SystemExit(1)
