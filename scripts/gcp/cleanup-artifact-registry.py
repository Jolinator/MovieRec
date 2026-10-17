#!/usr/bin/env python3
"""Delete all Artifact Registry repositories outside a selected location."""

import argparse
import json
import os
import subprocess
import sys
from typing import Any


def repository_location(repository: dict[str, Any]) -> str:
    location = repository.get("location")
    if isinstance(location, str) and location:
        return location

    name = repository.get("name", "")
    parts = name.split("/")
    try:
        location_index = parts.index("locations")
        return parts[location_index + 1]
    except (ValueError, IndexError) as error:
        raise ValueError(f"Cannot determine location for repository: {name!r}") from error


def repository_id(repository: dict[str, Any]) -> str:
    name = repository.get("name", "")
    repo_id = name.rsplit("/", 1)[-1]
    if not repo_id:
        raise ValueError(f"Cannot determine repository ID from: {name!r}")
    return repo_id


def list_repositories(project: str) -> list[dict[str, Any]]:
    result = subprocess.run(
        [
            "gcloud",
            "artifacts",
            "repositories",
            "list",
            f"--project={project}",
            "--location=all",
            "--format=json",
        ],
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode:
        if result.stderr:
            print(result.stderr, file=sys.stderr, end="")
        raise RuntimeError("Could not list Artifact Registry repositories.")

    repositories = json.loads(result.stdout or "[]")
    if not isinstance(repositories, list):
        raise ValueError("gcloud returned an unexpected repository list.")
    return repositories


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--keep-location", help="Artifact Registry location to preserve (defaults to GCP_ZONE's region)")
    parser.add_argument(
        "--project",
        default=os.environ.get("GCP_PROJECT_ID"),
        help="GCP project (defaults to GCP_PROJECT_ID from the Make configuration)",
    )
    args = parser.parse_args()

    project = args.project
    if not project:
        print("Set GCP_PROJECT_ID or pass --project.", file=sys.stderr)
        return 2

    keep_location = args.keep_location
    zone = os.environ.get("GCP_ZONE", "")
    if not keep_location:
        region, separator, zone_suffix = zone.rpartition("-")
        if not separator or not region or not zone_suffix:
            print("Cannot derive the Artifact Registry location; set GCP_ZONE or pass --keep-location.", file=sys.stderr)
            return 2
        keep_location = region

    try:
        repositories = list_repositories(project)
        # Resolve every location before printing or deleting anything. If the API
        # response is unfamiliar, fail closed rather than risk deleting a repo.
        planned = [
            (repository_id(repository), repository_location(repository))
            for repository in repositories
        ]
    except (RuntimeError, ValueError, json.JSONDecodeError) as error:
        print(str(error), file=sys.stderr)
        return 1

    to_delete = [(repo_id, location) for repo_id, location in planned if location != keep_location]
    print(f"Project: {project}")
    source = f" (derived from GCP_ZONE={zone})" if not args.keep_location else ""
    print(f"Keeping Artifact Registry location: {keep_location}{source}")
    if not to_delete:
        print("No repositories outside the keep location were found.")
        return 0

    print("Repositories to delete:")
    for repo_id, location in to_delete:
        print(f"  {location}/{repo_id}")
    print(f"This will permanently delete {len(to_delete)} Artifact Registry repositories and their contents.")
    try:
        confirmation = input('Type DELETE to continue: ')
    except EOFError:
        confirmation = ""
    if confirmation != "DELETE":
        print("Cancelled; no repositories were deleted.")
        return 1

    failures = 0
    for repo_id, location in to_delete:
        result = subprocess.run(
            [
                "gcloud",
                "artifacts",
                "repositories",
                "delete",
                repo_id,
                f"--project={project}",
                f"--location={location}",
                "--quiet",
            ],
            check=False,
            text=True,
        )
        if result.returncode:
            failures += 1
            print(f"Failed to delete {location}/{repo_id}.", file=sys.stderr)
        else:
            print(f"Deleted {location}/{repo_id}.")

    if failures:
        print(f"{failures} repository deletion(s) failed.", file=sys.stderr)
        return 1
    print("Artifact Registry cleanup complete.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
