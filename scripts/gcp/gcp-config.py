#!/usr/bin/env python3
"""Load GCP settings and read or update server.env and Terraform variables."""

import argparse
import json
import os
import re
import shlex
import shutil
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
SERVER_ENV = REPO_ROOT / "deploy/gcp/secrets/server.env"
TFVARS = REPO_ROOT / "deploy/gcp/terraform/terraform.tfvars"
TFVARS_EXAMPLE = REPO_ROOT / "deploy/gcp/terraform/terraform.tfvars.example"
GPU_TYPE_MACHINE_PREFIXES = {
    "nvidia-b200": "a4-highgpu-",
    "nvidia-h100-80gb": "a3-highgpu-",
    "nvidia-h100-mega-80gb": "a3-megagpu-",
    "nvidia-h200-141gb": "a3-ultragpu-",
    "nvidia-l4": "g2-",
    "nvidia-rtx-pro-6000": "g4-",
    "nvidia-tesla-t4": "n1-",
}


def read_server_env() -> dict[str, str]:
    values: dict[str, str] = {}
    if not SERVER_ENV.is_file():
        return values

    for line in SERVER_ENV.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if stripped.startswith("export "):
            stripped = stripped[7:].lstrip()

        try:
            tokens = shlex.split(stripped, comments=True)
        except ValueError as error:
            raise ValueError(f"Invalid line in {SERVER_ENV}: {error}") from error
        if not tokens:
            continue

        key, separator, value = tokens[0].partition("=")
        if separator and key.isidentifier():
            values[key] = value

    return values


def assignment_key(line: str) -> str | None:
    stripped = line.lstrip()
    if not stripped or stripped.startswith(("#", "//", "/*")):
        return None

    left, separator, _ = line.partition("=")
    return left.strip() if separator else None


def without_comment(value: str) -> str:
    in_string = False
    escaped = False

    for index, char in enumerate(value):
        if in_string:
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == '"':
                in_string = False
        elif char == '"':
            in_string = True
        elif char == "#" or (char == "/" and value[index : index + 2] == "//"):
            return value[:index]

    return value


def read_tfvars(keys: set[str]) -> dict[str, str]:
    values: dict[str, str] = {}
    if not TFVARS.is_file():
        return values

    for line in TFVARS.read_text(encoding="utf-8").splitlines():
        key = assignment_key(line)
        if key not in keys or key in values:
            continue

        _, _, raw_value = line.partition("=")
        raw_value = without_comment(raw_value).strip()
        if raw_value.startswith('"'):
            value = json.loads(raw_value)
            if not isinstance(value, str):
                raise ValueError(f"Terraform variable {key} must be a string")
            values[key] = value
        else:
            values[key] = raw_value

    return values


def first_value(*values: str | None) -> str:
    return next((value for value in values if value), "")


def resolve(
    name: str,
    file_env: dict[str, str],
    process_env: dict[str, str],
    tfvars: dict[str, str],
) -> str:
    if name in ("GCP_REGION", "TF_VAR_region"):
        return first_value(
            tfvars.get("region"),
            file_env.get("GCP_REGION"),
            process_env.get("GCP_REGION"),
            file_env.get("TF_VAR_region"),
            process_env.get("TF_VAR_region"),
            "europe-west1",
        )

    if name in ("GCP_ZONE", "TF_VAR_zone"):
        return first_value(
            tfvars.get("zone"),
            file_env.get("GCP_ZONE"),
            process_env.get("GCP_ZONE"),
            file_env.get("TF_VAR_zone"),
            process_env.get("TF_VAR_zone"),
            "europe-west1-b",
        )

    if name == "TF_VAR_project_id":
        return first_value(file_env.get(name), process_env.get(name))

    if name == "GCP_PROJECT_ID":
        return first_value(
            file_env.get(name),
            process_env.get(name),
            file_env.get("TF_VAR_project_id"),
            process_env.get("TF_VAR_project_id"),
        )

    if name in ("MOVIEREC_RESOURCE_NAME", "TF_VAR_name"):
        return first_value(
            file_env.get("MOVIEREC_RESOURCE_NAME"),
            process_env.get("MOVIEREC_RESOURCE_NAME"),
            file_env.get("TF_VAR_name"),
            process_env.get("TF_VAR_name"),
            "movierec",
        )

    return first_value(file_env.get(name), process_env.get(name))


def write_tfvar(key: str, value: str, *, raw: bool = False) -> None:
    if not TFVARS.is_file():
        raise FileNotFoundError(f"Terraform variables file does not exist: {TFVARS}")

    with TFVARS.open("r", encoding="utf-8", newline="") as tfvars_file:
        lines = tfvars_file.readlines()

    serialized = value if raw else json.dumps(value)
    replacement = f"{key} = {serialized}"
    found = False
    updated = []

    for line in lines:
        if assignment_key(line) == key:
            newline = "\r\n" if line.endswith("\r\n") else "\n" if line.endswith("\n") else ""
            indent = line[: len(line) - len(line.lstrip())]
            updated.append(f"{indent}{replacement}{newline}")
            found = True
        else:
            updated.append(line)

    if not found:
        if updated and not updated[-1].endswith(("\n", "\r")):
            updated[-1] += "\n"
        updated.append(f"{replacement}\n")

    with TFVARS.open("w", encoding="utf-8", newline="") as tfvars_file:
        tfvars_file.write("".join(updated))


def write_server_env(key: str, value: str) -> None:
    valid_start = key[:1].isalpha() or key.startswith("_")
    if not valid_start or not all(char.isalnum() or char == "_" for char in key):
        raise ValueError(f"Invalid environment variable name: {key}")

    if SERVER_ENV.is_file():
        with SERVER_ENV.open("r", encoding="utf-8", newline="") as env_file:
            lines = env_file.readlines()
    else:
        lines = []

    replacement = f"{key}={shlex.quote(value)}"
    found = False
    updated = []

    for line in lines:
        stripped = line.lstrip()
        candidate = stripped[7:].lstrip() if stripped.startswith("export ") else stripped
        left, separator, _ = candidate.partition("=")
        if separator and left.strip() == key:
            newline = "\r\n" if line.endswith("\r\n") else "\n" if line.endswith("\n") else ""
            indent = line[: len(line) - len(stripped)]
            export = "export " if stripped.startswith("export ") else ""
            updated.append(f"{indent}{export}{replacement}{newline}")
            found = True
        else:
            updated.append(line)

    if not found:
        if updated and not updated[-1].endswith(("\n", "\r")):
            updated[-1] += "\n"
        updated.append(f"{replacement}\n")

    with SERVER_ENV.open("w", encoding="utf-8", newline="") as env_file:
        env_file.write("".join(updated))


def validate_gpu_options(mode: str, gpu_type: str, machine_type: str, count: int) -> str:
    """Validate GPU options and return the selected mode for Make."""
    if mode not in {"auto", "on", "off"}:
        raise ValueError("GPU must be one of: auto, on, off")

    if mode != "on":
        return mode

    base_type = gpu_type[:-4] if gpu_type.endswith("-vws") else gpu_type
    if base_type not in GPU_TYPE_MACHINE_PREFIXES:
        supported = ", ".join(sorted(GPU_TYPE_MACHINE_PREFIXES))
        raise ValueError(f"Unsupported GPU_TYPE '{gpu_type}'. Choose one of: {supported}")
    if count not in {1, 2, 4}:
        raise ValueError("GPU_COUNT must be 1, 2, or 4")
    if base_type != "nvidia-tesla-t4" and count != 1:
        raise ValueError("GPU_COUNT applies only to N1-attached T4 GPUs; other types get their count from GPU_MACHINE_TYPE")

    expected_prefix = GPU_TYPE_MACHINE_PREFIXES[base_type]
    if machine_type and not re.fullmatch(r"[a-z][a-z0-9-]*", machine_type):
        raise ValueError("GPU_MACHINE_TYPE must contain only lowercase letters, numbers, and hyphens")
    if machine_type and not machine_type.startswith(expected_prefix):
        raise ValueError(f"{gpu_type} requires a machine type beginning with {expected_prefix}")

    return mode


def set_gpu_tfvars(mode: str, gpu_type: str, machine_type: str, count: int) -> None:
    """Persist an explicit GPU on/off choice in terraform.tfvars."""
    validate_gpu_options(mode, gpu_type, machine_type, count)
    if mode == "auto":
        print("GPU settings unchanged (GPU=auto).")
        return

    if not TFVARS.is_file():
        shutil.copyfile(TFVARS_EXAMPLE, TFVARS)
    os.chmod(TFVARS, 0o600)

    write_tfvar("model_gpu_enabled", "true" if mode == "on" else "false", raw=True)
    if mode == "on":
        write_tfvar("model_gpu_type", gpu_type)
        write_tfvar("model_gpu_machine_type", machine_type or "null", raw=not machine_type)
        write_tfvar("model_gpu_count", str(count), raw=True)
    print(f"Saved GPU={mode} settings to {TFVARS}.")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)

    commands.add_parser("make-vars", help="print resolved values for Make to import")

    set_parser = commands.add_parser("set-tfvar", help="set a Terraform variable")
    set_parser.add_argument("key")
    set_parser.add_argument("value")
    set_parser.add_argument("--raw", action="store_true", help="write value as a Terraform expression")

    set_env_parser = commands.add_parser("set-server-env", help="set a value in the server environment file")
    set_env_parser.add_argument("key")
    set_env_parser.add_argument("value")

    gpu_parser = commands.add_parser("gpu-mode", help="validate GPU options and print the selected mode for Make")
    gpu_parser.add_argument("--mode", required=True)
    gpu_parser.add_argument("--type", default="nvidia-tesla-t4")
    gpu_parser.add_argument("--machine-type", default="")
    gpu_parser.add_argument("--count", type=int, default=1)

    set_gpu_parser = commands.add_parser("set-gpu-vars", help="persist an explicit GPU choice in terraform.tfvars")
    set_gpu_parser.add_argument("--mode", required=True)
    set_gpu_parser.add_argument("--type", default="nvidia-tesla-t4")
    set_gpu_parser.add_argument("--machine-type", default="")
    set_gpu_parser.add_argument("--count", type=int, default=1)

    args = parser.parse_args()
    if args.command == "make-vars":
        file_env = read_server_env()
        process_env = dict(os.environ)
        tfvars = read_tfvars({"region", "zone"})
        names = (
            "TF_VAR_project_id",
            "GCP_PROJECT_ID",
            "GCP_REGION",
            "GCP_ZONE",
            "MOVIEREC_RESOURCE_NAME",
        )
        for name in names:
            value = resolve(name, file_env, process_env, tfvars)
            if any(char.isspace() or char in "#$=," for char in value):
                raise ValueError(f"{name} contains characters that cannot be imported by Make")
            print(f"{name}={value}")
    elif args.command == "set-tfvar":
        write_tfvar(args.key, args.value, raw=args.raw)
    elif args.command == "gpu-mode":
        try:
            print(validate_gpu_options(args.mode, args.type, args.machine_type, args.count))
        except ValueError as error:
            parser.error(str(error))
    elif args.command == "set-gpu-vars":
        try:
            set_gpu_tfvars(args.mode, args.type, args.machine_type, args.count)
        except ValueError as error:
            parser.error(str(error))
    else:
        write_server_env(args.key, args.value)


if __name__ == "__main__":
    main()
