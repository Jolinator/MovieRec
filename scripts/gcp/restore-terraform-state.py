#!/usr/bin/env python3
"""Restore a validated Terraform state generation from the shared GCS backend."""

from __future__ import annotations

import argparse
import json
import os
import re
import shlex
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]
TF_DIR = REPO_ROOT / "deploy" / "gcp" / "terraform"
BACKEND_FILE = TF_DIR / "backend.tf"
BACKEND_CACHE = TF_DIR / ".terraform" / "terraform.tfstate"
STATE_REQUIREMENTS_FILE = Path(__file__).with_name("terraform-state-requirements.json")

with STATE_REQUIREMENTS_FILE.open(encoding="utf-8") as requirements_file:
    STATE_REQUIREMENTS = json.load(requirements_file)

STATE_PREFIX = STATE_REQUIREMENTS["state_prefix"]
MIN_STATE_RESOURCES = STATE_REQUIREMENTS["minimum_resources"]
REQUIRED_STATE_RESOURCES = set(STATE_REQUIREMENTS["required_resources"])


class RestoreError(Exception):
    pass


def run(argv: list[str], *, capture: bool = False) -> str:
    try:
        result = subprocess.run(
            argv,
            check=False,
            text=True,
            stdout=subprocess.PIPE if capture else None,
            stderr=subprocess.PIPE if capture else None,
        )
    except FileNotFoundError as error:
        raise RestoreError(f"Required command not found: {argv[0]}") from error

    if result.returncode != 0:
        detail = (result.stderr or "").strip() if capture else ""
        suffix = f"\n{detail}" if detail else ""
        raise RestoreError(f"Command failed: {shlex.join(argv)}{suffix}")
    return (result.stdout or "") if capture else ""


def read_backend_bucket() -> str:
    if not BACKEND_FILE.is_file():
        raise RestoreError(f"{BACKEND_FILE} is missing; join the shared backend first.")
    contents = BACKEND_FILE.read_text(encoding="utf-8")
    if not re.search(r'backend\s+"gcs"\s*\{', contents):
        raise RestoreError(f"{BACKEND_FILE} does not configure the GCS backend.")

    buckets = re.findall(r'^\s*bucket\s*=\s*"([^"]+)"\s*$', contents, re.MULTILINE)
    prefixes = re.findall(r'^\s*prefix\s*=\s*"([^"]+)"\s*$', contents, re.MULTILINE)
    if len(buckets) != 1 or len(prefixes) != 1:
        raise RestoreError(f"Could not read one bucket and prefix from {BACKEND_FILE}.")
    if prefixes[0] != STATE_PREFIX:
        raise RestoreError(
            f"Expected state prefix {STATE_PREFIX!r}, found {prefixes[0]!r}; refusing restore."
        )
    return buckets[0]


def verify_active_backend(bucket: str) -> None:
    if not BACKEND_CACHE.is_file():
        raise RestoreError("Terraform is not initialized. Run 'make tf-backend-join' first.")
    try:
        cache = json.loads(BACKEND_CACHE.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise RestoreError("Could not read Terraform's cached backend configuration.") from error

    backend = cache.get("backend", {})
    config = backend.get("config", {})
    if backend.get("type") != "gcs":
        raise RestoreError("This shell is not initialized with the GCS backend; run 'make tf-backend-join'.")
    if config.get("bucket") != bucket or config.get("prefix") != STATE_PREFIX:
        raise RestoreError(
            "The initialized backend does not match backend.tf; run 'make tf-backend-join' and retry."
        )

    workspace = run(["terraform", f"-chdir={TF_DIR}", "workspace", "show"], capture=True).strip()
    if workspace != "default":
        raise RestoreError(f"Expected Terraform workspace 'default', found {workspace!r}.")

    workspaces = run(["terraform", f"-chdir={TF_DIR}", "workspace", "list"], capture=True)
    names = [line.strip().lstrip("*").strip() for line in workspaces.splitlines() if line.strip()]
    if names != ["default"]:
        raise RestoreError(f"Expected only the default workspace; found: {', '.join(names)}.")


def parse_state_addresses(state_path: Path) -> list[str]:
    try:
        state = json.loads(state_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise RestoreError("The selected GCS object is not a readable Terraform state file.") from error

    addresses: list[str] = []
    resources = state.get("resources")
    if not isinstance(resources, list):
        raise RestoreError("The selected object has no Terraform resource list.")

    for resource in resources:
        if not isinstance(resource, dict):
            continue
        module = resource.get("module")
        prefix = f"{module}." if module else ""
        if resource.get("mode") == "data":
            prefix += "data."
        base = f"{prefix}{resource.get('type', '')}.{resource.get('name', '')}"
        instances = resource.get("instances", [])
        if not isinstance(instances, list):
            continue
        for instance in instances:
            address = base
            if isinstance(instance, dict) and "index_key" in instance:
                key = instance["index_key"]
                if isinstance(key, str):
                    address += f"[{json.dumps(key, ensure_ascii=False)}]"
                elif isinstance(key, (int, float)) and not isinstance(key, bool):
                    address += f"[{key}]"
            addresses.append(address)
    return addresses


def inspect_state(state_path: Path) -> tuple[int, list[str]]:
    addresses = parse_state_addresses(state_path)
    missing = sorted(REQUIRED_STATE_RESOURCES.difference(addresses))
    return len(addresses), missing


def object_versions(object_url: str) -> list[tuple[str, str]]:
    listing = run(["gcloud", "storage", "ls", "--all-versions", object_url], capture=True)
    versions: list[tuple[str, str]] = []
    for line in listing.splitlines():
        url = line.strip()
        if not url.startswith(object_url + "#"):
            continue
        generation = url.rsplit("#", 1)[1]
        if generation.isdigit():
            versions.append((generation, url))
    if not versions:
        raise RestoreError(f"No versioned state object found at {object_url}.")
    return versions


def version_details(version_url: str) -> tuple[str, str]:
    raw = run(["gcloud", "storage", "objects", "describe", version_url, "--format=json"], capture=True)
    try:
        info = json.loads(raw)
    except json.JSONDecodeError as error:
        raise RestoreError(f"Could not read metadata for {version_url}.") from error
    created = str(info.get("timeCreated", "unknown time"))
    size = str(info.get("size", "unknown size"))
    return created, size


def download_version(version_url: str, target: Path) -> None:
    run(["gcloud", "storage", "cp", "--quiet", version_url, str(target)])
    target.chmod(0o600)


def save_current_state_backup() -> Path:
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    backup_path = TF_DIR / f"terraform.pre-restore-{timestamp}-{os.getpid()}.tfstate"
    try:
        state = run(["terraform", f"-chdir={TF_DIR}", "state", "pull"], capture=True)
        descriptor = os.open(backup_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "w", encoding="utf-8") as backup_file:
            backup_file.write(state)
    except FileExistsError:
        raise RestoreError(f"Backup file already exists: {backup_path}")
    except OSError as error:
        raise RestoreError(f"Could not save the current state backup: {error}") from error
    return backup_path


def choose_generation(
    full_versions: list[dict[str, Any]], requested_generation: str | None
) -> dict[str, Any]:
    if requested_generation is not None:
        for version in full_versions:
            if version["generation"] == requested_generation:
                return version
        raise RestoreError(
            f"Generation {requested_generation} is unavailable or does not contain the expected full state."
        )

    if not sys.stdin.isatty():
        raise RestoreError("Use --generation from a terminal to select a backup non-interactively.")

    print("Full-state backups:")
    for index, version in enumerate(full_versions, start=1):
        print(
            f"  {index}) generation {version['generation']} | {version['created']} | "
            f"{version['size']} bytes | {version['resources']} resources"
        )
    while True:
        answer = input("Choose a backup number (or type 'stop'): ").strip().lower()
        if answer in {"stop", "cancel", "q"}:
            raise RestoreError("Restore cancelled.")
        if answer.isdigit() and 1 <= int(answer) <= len(full_versions):
            return full_versions[int(answer) - 1]
        print("Enter one of the listed numbers or type 'stop'.")


def restore(requested_generation: str | None) -> None:
    if not sys.stdin.isatty():
        raise RestoreError("Restore requires an interactive terminal for the final confirmation.")

    bucket = read_backend_bucket()
    verify_active_backend(bucket)

    bucket_url = f"gs://{bucket}"
    bucket_info = run(["gcloud", "storage", "buckets", "describe", bucket_url, "--format=json"], capture=True)
    try:
        versioning_enabled = json.loads(bucket_info).get("versioning", {}).get("enabled") is True
    except (json.JSONDecodeError, AttributeError) as error:
        raise RestoreError(f"Could not verify object versioning for {bucket_url}.") from error
    if not versioning_enabled:
        raise RestoreError(f"Object versioning is not enabled on {bucket_url}; refusing a state restore.")

    object_url = f"{bucket_url}/{STATE_PREFIX}/default.tfstate"
    versions = object_versions(object_url)
    full_versions: list[dict[str, Any]] = []
    with tempfile.TemporaryDirectory(prefix="terraform-state-restore-") as temporary_directory:
        temporary_dir = Path(temporary_directory)
        temporary_dir.chmod(0o700)
        state_path = temporary_dir / "candidate.tfstate"

        print(f"Checking {len(versions)} state versions at {object_url}.")
        for generation, version_url in versions:
            download_version(version_url, state_path)
            count, missing = inspect_state(state_path)
            created, size = version_details(version_url)
            if count >= MIN_STATE_RESOURCES and not missing:
                full_versions.append(
                    {
                        "generation": generation,
                        "url": version_url,
                        "created": created,
                        "size": size,
                        "resources": count,
                    }
                )
            else:
                details = f"missing {', '.join(missing)}" if missing else f"only {count} resources"
                print(f"  Skipping generation {generation} ({details}).")

        if not full_versions:
            raise RestoreError("No backup passed the full-state checks; nothing was changed.")

        selected = choose_generation(full_versions, requested_generation)
        download_version(selected["url"], state_path)
        count, missing = inspect_state(state_path)
        if count < MIN_STATE_RESOURCES or missing:
            raise RestoreError("Selected backup failed validation; nothing was changed.")

        current_list = run(["terraform", f"-chdir={TF_DIR}", "state", "list"], capture=True)
        current_count = len([line for line in current_list.splitlines() if line.strip()])
        print(
            f"\nSelected generation {selected['generation']} from {selected['created']} "
            f"({count} resources). Current shared state has {current_count} resources."
        )
        backup_path = save_current_state_backup()
        print(f"Private backup of current state: {backup_path}")
        print("Confirm all admins have stopped Terraform operations before restoring.")
        print("This replaces the active shared state. The selected backup passed all required-resource checks.")
        confirmation = input("Type RESTORE to continue: ").strip()
        if confirmation != "RESTORE":
            raise RestoreError("Restore cancelled; the state backup was kept.")

        # Terraform's state push takes the backend lock; -force is needed when
        # restoring a historical snapshot with a different lineage or serial.
        run(["terraform", f"-chdir={TF_DIR}", "state", "push", "-force", str(state_path)])

        restored_list = run(["terraform", f"-chdir={TF_DIR}", "state", "list"], capture=True)
        restored_count = len([line for line in restored_list.splitlines() if line.strip()])
        restored_addresses = set(restored_list.splitlines())
        missing_after_push = sorted(REQUIRED_STATE_RESOURCES.difference(restored_addresses))
        if restored_count < MIN_STATE_RESOURCES or missing_after_push:
            raise RestoreError(
                "State push completed, but the live state failed post-restore validation. "
                f"Stop Terraform operations and inspect the saved backup {backup_path}."
            )
        print(f"Restored shared Terraform state ({restored_count} resources).")
        print("All admins can reconnect or run state list to confirm the restored state.")


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Restore a validated full Terraform state snapshot from GCS object history."
    )
    parser.add_argument(
        "--generation",
        help="Restore this GCS generation instead of choosing interactively; confirmation is still required.",
    )
    args = parser.parse_args()
    if args.generation and not args.generation.isdigit():
        parser.error("--generation must be a numeric GCS generation ID")

    try:
        restore(args.generation)
    except RestoreError as error:
        print(f"terraform-backend restore: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
