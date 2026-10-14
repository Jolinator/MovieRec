#!/usr/bin/env python3
"""Set or reset Docker's maximum concurrent image uploads."""

import json
import os
import re
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
from pathlib import Path


DAEMON_CONFIG = Path("/etc/docker/daemon.json")
SETTING = "max-concurrent-uploads"


def write_config(config: dict[str, object], mode: int) -> None:
    temporary_path: Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w",
            encoding="utf-8",
            dir=DAEMON_CONFIG.parent,
            prefix=f".{DAEMON_CONFIG.name}.",
            delete=False,
        ) as temporary_file:
            json.dump(config, temporary_file, indent=2)
            temporary_file.write("\n")
            temporary_path = Path(temporary_file.name)
        os.chmod(temporary_path, mode)
        os.replace(temporary_path, DAEMON_CONFIG)
    finally:
        if temporary_path is not None and temporary_path.exists():
            temporary_path.unlink()


def load_config() -> tuple[dict[str, object], int]:
    if not DAEMON_CONFIG.exists():
        return {}, 0o644

    try:
        config = json.loads(DAEMON_CONFIG.read_text(encoding="utf-8"))
    except json.JSONDecodeError as error:
        raise ValueError(f"Invalid JSON in {DAEMON_CONFIG}: {error}") from error
    if not isinstance(config, dict):
        raise ValueError(f"Expected a JSON object in {DAEMON_CONFIG}")
    return config, stat.S_IMODE(DAEMON_CONFIG.stat().st_mode)


def reload_docker(action: str, value: str = "") -> int:
    if action == "set":
        failure = f"Updated {DAEMON_CONFIG}, but could not find the dockerd process to reload it."
    else:
        failure = f"Removed the setting from {DAEMON_CONFIG}, but could not find dockerd to reload it."

    try:
        result = subprocess.run(
            ["pgrep", "-xo", "dockerd"], check=True, capture_output=True, text=True
        )
        dockerd_pid = int(result.stdout.strip())
        os.kill(dockerd_pid, signal.SIGHUP)
    except (OSError, ValueError, subprocess.CalledProcessError):
        print(failure, file=sys.stderr)
        return 1

    if action == "set":
        print(f"Set {SETTING} to {value} and reloaded dockerd.")
    else:
        print(f"Removed {SETTING} and reloaded dockerd; Docker will use its default.")
    return 0


def main() -> int:
    if os.geteuid() != 0:
        print(
            f"Run this helper with sudo, for example: sudo python3 {sys.argv[0]} set 1",
            file=sys.stderr,
        )
        return 2

    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} set POSITIVE_INTEGER | reset", file=sys.stderr)
        return 2

    action = sys.argv[1]
    if action == "set":
        if len(sys.argv) != 3:
            print(f"Usage: {sys.argv[0]} set POSITIVE_INTEGER", file=sys.stderr)
            return 2
        value = sys.argv[2]
        if not re.fullmatch(r"[1-9][0-9]*", value):
            print(f"Upload limit must be a positive integer; got: {value}", file=sys.stderr)
            return 2
    elif action == "reset":
        if len(sys.argv) != 2:
            print(f"Usage: {sys.argv[0]} reset", file=sys.stderr)
            return 2
    else:
        print(f"Unknown action: {action}", file=sys.stderr)
        print(f"Usage: {sys.argv[0]} set POSITIVE_INTEGER | reset", file=sys.stderr)
        return 2

    if shutil.which("pgrep") is None:
        print("Required command not found: pgrep", file=sys.stderr)
        return 1

    try:
        config, mode = load_config()
        if action == "set":
            DAEMON_CONFIG.parent.mkdir(parents=True, exist_ok=True)
            config[SETTING] = int(value)
            write_config(config, mode)
        else:
            if not DAEMON_CONFIG.exists():
                print(f"{DAEMON_CONFIG} does not exist; Docker is already using its default upload limit.")
                return 0
            if SETTING not in config:
                print(f"{SETTING} is not set in {DAEMON_CONFIG}; no change needed.")
                return 0
            del config[SETTING]
            if not config:
                DAEMON_CONFIG.unlink()
            else:
                write_config(config, mode)
    except (OSError, ValueError) as error:
        print(error, file=sys.stderr)
        return 1

    return reload_docker(action, value if action == "set" else "")


if __name__ == "__main__":
    raise SystemExit(main())
