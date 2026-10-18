#!/usr/bin/env python3
"""Check that an existing model data disk is safe for automatic import."""

import json
import sys


def last_component(value: str) -> str:
    return value.rstrip("/").rsplit("/", 1)[-1]


def main() -> int:
    if len(sys.argv) != 4:
        print("Usage: validate-model-data-disk.py NAME ZONE SIZE_GB")
        return 2

    expected_name, expected_zone, expected_size = sys.argv[1:]
    try:
        disk = json.load(sys.stdin)
    except json.JSONDecodeError as error:
        print(f"invalid disk JSON: {error}")
        return 2

    problems = []
    if disk.get("name") != expected_name:
        problems.append(f"name is {disk.get('name')!r}")
    if last_component(disk.get("zone", "")) != expected_zone:
        problems.append(f"zone is {disk.get('zone')!r}")
    if last_component(disk.get("type", "")) != "pd-balanced":
        problems.append(f"type is {disk.get('type')!r}, expected pd-balanced")
    if str(disk.get("sizeGb", "")) != expected_size:
        problems.append(f"size is {disk.get('sizeGb')!r} GB, expected {expected_size} GB")
    if disk.get("status") != "READY":
        problems.append(f"status is {disk.get('status')!r}, expected READY")
    if disk.get("users"):
        problems.append("it is attached to an instance")

    if problems:
        print("; ".join(problems))
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
