#!/usr/bin/env python3
"""Fill database password placeholders in deploy/gcp/secrets/*.env safely."""

from __future__ import annotations

import os
import secrets
import stat
import sys
import tempfile
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
SECRETS_DIR = REPO_ROOT / "deploy" / "gcp" / "secrets"
PASSWORD_PAIRS = {
    "auth.env": ("POSTGRES_PASSWORD", "AUTH_DB_PASSWORD"),
    "movies.env": ("POSTGRES_PASSWORD", "MOVIES_DB_PASSWORD"),
    "recommendations.env": ("POSTGRES_PASSWORD", "RECOMMENDATIONS_DB_PASSWORD"),
}
PASSWORD_KEYS = {key for pair in PASSWORD_PAIRS.values() for key in pair}


def read_env(path: Path) -> tuple[list[str], dict[str, tuple[int, str]]]:
    try:
        lines = path.read_text(encoding="utf-8").splitlines(keepends=True)
    except FileNotFoundError:
        raise SystemExit(f"Missing {path}; run make secrets-init first.")

    assignments: dict[str, tuple[int, str]] = {}
    for index, line in enumerate(lines):
        body = line.rstrip("\r\n")
        key, separator, value = body.partition("=")
        if separator and key.strip() in PASSWORD_KEYS:
            normalized_key = key.strip()
            if normalized_key in assignments:
                raise SystemExit(f"{path} contains {normalized_key} more than once; fix it manually.")
            assignments[normalized_key] = (index, value)
    return lines, assignments


def replace_value(lines: list[str], index: int, key: str, value: str) -> None:
    line = lines[index]
    ending = line[len(line.rstrip("\r\n")) :]
    lines[index] = f"{key}={value}{ending}"


def write_secret_file(path: Path, lines: list[str]) -> None:
    temp_path: Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w",
            encoding="utf-8",
            dir=path.parent,
            prefix=f".{path.name}.",
            delete=False,
        ) as temp_file:
            temp_file.writelines(lines)
            temp_file.flush()
            os.fsync(temp_file.fileno())
            temp_path = Path(temp_file.name)
        os.chmod(temp_path, stat.S_IRUSR | stat.S_IWUSR)
        os.replace(temp_path, path)
    finally:
        if temp_path is not None and temp_path.exists():
            temp_path.unlink()


def is_unset(value: str) -> bool:
    return not value.strip() or "REPLACE_" in value


def main() -> int:
    plans: list[tuple[Path, list[str], bool]] = []
    updated_services: list[str] = []

    for filename, (postgres_key, app_key) in PASSWORD_PAIRS.items():
        path = SECRETS_DIR / filename
        lines, assignments = read_env(path)
        missing = [key for key in (postgres_key, app_key) if key not in assignments]
        if missing:
            raise SystemExit(f"{path} is missing required key(s): {', '.join(missing)}")

        postgres_index, postgres_value = assignments[postgres_key]
        app_index, app_value = assignments[app_key]
        postgres_unset = is_unset(postgres_value)
        app_unset = is_unset(app_value)

        if not postgres_unset and not app_unset:
            if postgres_value != app_value:
                raise SystemExit(
                    f"{path} has different existing values for {postgres_key} and {app_key}; "
                    "refusing to change database credentials."
                )
            plans.append((path, lines, False))
            continue

        if postgres_unset and app_unset:
            password = secrets.token_hex(32)
        elif postgres_unset:
            password = app_value
        else:
            password = postgres_value

        replace_value(lines, postgres_index, postgres_key, password)
        replace_value(lines, app_index, app_key, password)
        plans.append((path, lines, True))
        updated_services.append(filename.removesuffix(".env"))

    # Validate all three pairs before writing any of the files.
    for path, lines, changed in plans:
        if changed:
            write_secret_file(path, lines)

    if updated_services:
        print("Generated or matched passwords for: " + ", ".join(updated_services))
    else:
        print("Database passwords are already set and matched; no files changed.")

    tmdb_path = SECRETS_DIR / "tmdb.env"
    if tmdb_path.is_file() and "REPLACE_" in tmdb_path.read_text(encoding="utf-8"):
        print("Set TMDB_READ_ACCESS_TOKEN in tmdb.env manually; it cannot be generated locally.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except OSError as error:
        print(f"Could not update secret files: {error}", file=sys.stderr)
        raise SystemExit(1)
