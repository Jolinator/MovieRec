#!/usr/bin/env python3
"""Load GCP settings and read or update server.env and Terraform variables."""

import argparse
import json
import os
import re
import shlex
import shutil
import subprocess
import sys
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
SUPPORTED_GPU_TYPES = set(GPU_TYPE_MACHINE_PREFIXES) | {
    "nvidia-l4-vws",
    "nvidia-rtx-pro-6000-vws",
    "nvidia-tesla-t4-vws",
}
GPU_MACHINE_FAMILY_TYPES = {
    prefix: gpu_type
    for gpu_type, prefix in GPU_TYPE_MACHINE_PREFIXES.items()
    if gpu_type != "nvidia-tesla-t4"
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
        )

    if name in ("GCP_ZONE", "TF_VAR_zone"):
        return first_value(
            tfvars.get("zone"),
            file_env.get("GCP_ZONE"),
            process_env.get("GCP_ZONE"),
            file_env.get("TF_VAR_zone"),
            process_env.get("TF_VAR_zone"),
        )

    if name == "GCP_ARTIFACT_REGISTRY_REGION":
        artifact_region = tfvars.get("artifact_registry_region")
        if artifact_region in {None, "", "null"}:
            artifact_region = None
        return first_value(
            artifact_region,
            tfvars.get("region"),
            file_env.get(name),
            process_env.get(name),
        )

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
            tfvars.get("name"),
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
    if any(char.isspace() or char in "#$=," for char in value):
        raise ValueError(f"{key} contains characters that cannot be imported by Make")

    if SERVER_ENV.is_file():
        with SERVER_ENV.open("r", encoding="utf-8", newline="") as env_file:
            lines = env_file.readlines()
    else:
        lines = []

    replacement = f"{key}={value}"
    found = False
    updated = []

    for line in lines:
        stripped = line.lstrip()
        if server_env_key(line) == key:
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


def server_env_key(line: str) -> str | None:
    stripped = line.lstrip()
    candidate = stripped[7:].lstrip() if stripped.startswith("export ") else stripped
    left, separator, _ = candidate.partition("=")
    return left.strip() if separator else None


def remove_server_env(key: str) -> None:
    if not SERVER_ENV.is_file():
        return

    with SERVER_ENV.open("r", encoding="utf-8", newline="") as env_file:
        lines = env_file.readlines()

    retained = []
    for line in lines:
        if server_env_key(line) == key:
            continue
        retained.append(line)

    with SERVER_ENV.open("w", encoding="utf-8", newline="") as env_file:
        env_file.write("".join(retained))


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


def terraform_flags(operation: str, model_only: str, auto_approve: str) -> str:
    """Validate Make's Terraform switches and return flags for one operation."""
    if model_only not in {"0", "1"}:
        raise ValueError("MODEL_ONLY must be 0 or 1")
    if auto_approve not in {"0", "1"}:
        raise ValueError("TF_AUTO_APPROVE must be 0 or 1")

    flags: list[str] = []
    if model_only == "1":
        flags.extend(
            (
                "-target=google_compute_instance.model",
                "-target=google_compute_firewall.k3s_api",
                "-target=google_compute_firewall.k3s_flannel",
                "-target=google_compute_firewall.k3s_kubelet",
            )
        )
    if operation == "apply" and auto_approve == "1":
        flags.append("-auto-approve")

    return " ".join(flags) or "none"


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


def gcloud_value(*arguments: str) -> str:
    try:
        result = subprocess.run(
            ["gcloud", *arguments],
            check=False,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
        )
    except FileNotFoundError as error:
        raise RuntimeError("gcloud is required; run this command from Google Cloud Shell.") from error
    if result.returncode != 0:
        return ""
    return result.stdout.strip()


def gcloud_instances(project_id: str, instance_name: str) -> list[dict[str, object]]:
    try:
        result = subprocess.run(
            [
                "gcloud", "compute", "instances", "list",
                f"--project={project_id}",
                f"--filter=name={instance_name}",
                "--format=json",
                "--quiet",
            ],
            check=False,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    except FileNotFoundError as error:
        raise RuntimeError("gcloud is required; run this command from Google Cloud Shell.") from error
    if result.returncode != 0:
        raise RuntimeError(f"Could not inspect {instance_name}:\n{result.stderr.strip()}")

    try:
        instances = json.loads(result.stdout or "[]")
    except json.JSONDecodeError as error:
        raise RuntimeError(f"Could not parse the instance details for {instance_name}.") from error
    return [
        instance
        for instance in instances
        if isinstance(instance, dict) and instance.get("name") == instance_name
    ]


def resource_basename(value: object) -> str:
    return str(value or "").rstrip("/").rsplit("/", 1)[-1]


def model_gpu_settings(instance: dict[str, object]) -> dict[str, object]:
    machine_type = resource_basename(instance.get("machineType"))
    if not machine_type:
        raise ValueError("The model VM description does not include a machine type.")

    accelerators = instance.get("guestAccelerators") or []
    if not isinstance(accelerators, list):
        raise ValueError("The model VM has an unrecognized guest accelerator description.")

    gpu_type = ""
    gpu_count = 1
    if accelerators:
        if len(accelerators) != 1 or not isinstance(accelerators[0], dict):
            raise ValueError("The model VM has an unsupported guest accelerator configuration.")
        accelerator = accelerators[0]
        gpu_type = resource_basename(accelerator.get("acceleratorType"))
        if gpu_type not in SUPPORTED_GPU_TYPES:
            raise ValueError(f"The model VM uses unsupported GPU type {gpu_type!r}; set GPU settings manually.")
        if gpu_type.removesuffix("-vws") == "nvidia-tesla-t4":
            gpu_count = int(accelerator.get("acceleratorCount", accelerator.get("count", 1)))
            if gpu_count not in {1, 2, 4}:
                raise ValueError(f"The model VM has unsupported T4 GPU count {gpu_count}.")
    else:
        gpu_type = next(
            (kind for prefix, kind in GPU_MACHINE_FAMILY_TYPES.items() if machine_type.startswith(prefix)),
            "",
        )

    if not gpu_type:
        metadata = instance.get("metadata")
        metadata_items = metadata.get("items", []) if isinstance(metadata, dict) else []
        gpu_metadata = next(
            (
                str(item.get("value", ""))
                for item in metadata_items
                if isinstance(item, dict) and item.get("key") == "gpu_enabled"
            ),
            "",
        )
        if gpu_metadata.lower() == "true":
            raise ValueError(
                f"The model VM is marked GPU-enabled but its type cannot be inferred from {machine_type}."
            )
        return {"enabled": False, "machine_type": machine_type}

    base_type = gpu_type.removesuffix("-vws")
    if not machine_type.startswith(GPU_TYPE_MACHINE_PREFIXES[base_type]):
        raise ValueError(
            f"The model VM machine type {machine_type} is incompatible with {gpu_type}; set GPU settings manually."
        )
    return {
        "enabled": True,
        "gpu_type": gpu_type,
        "machine_type": machine_type,
        "gpu_count": gpu_count,
    }


def ensure_tfvars() -> None:
    """Create terraform.tfvars or add newly documented example settings without overwriting values."""
    if not TFVARS.is_file():
        shutil.copyfile(TFVARS_EXAMPLE, TFVARS)
        return

    existing_lines = TFVARS.read_text(encoding="utf-8").splitlines(keepends=True)
    existing_keys = {
        key for line in existing_lines if (key := assignment_key(line)) is not None
    }
    additions = [
        line if line.endswith(("\n", "\r")) else f"{line}\n"
        for line in TFVARS_EXAMPLE.read_text(encoding="utf-8").splitlines(keepends=True)
        if (key := assignment_key(line)) is not None and key not in existing_keys
    ]
    if additions:
        if existing_lines and not existing_lines[-1].endswith(("\n", "\r")):
            existing_lines[-1] += "\n"
        TFVARS.write_text("".join(existing_lines + additions), encoding="utf-8")


def glean_vars() -> None:
    """Discover the active project and app VM zone, then persist Make/Terraform settings."""
    ensure_tfvars()
    os.chmod(TFVARS, 0o600)

    file_env = read_server_env()
    process_env = dict(os.environ)
    tfvars = read_tfvars(
        {
            "name",
            "zone",
            "model_zone",
            "model_machine_type",
            "model_cpu_machine_type",
            "model_gpu_enabled",
            "model_gpu_type",
            "model_gpu_machine_type",
            "model_gpu_count",
            "artifact_registry_region",
        }
    )

    if not SERVER_ENV.is_file():
        SERVER_ENV.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(REPO_ROOT / "deploy/gcp/secrets-examples/server.env.example", SERVER_ENV)
        os.chmod(SERVER_ENV, 0o600)
        file_env = read_server_env()
        print(f"Created {SERVER_ENV} from its example.")

    project_id = gcloud_value("config", "get-value", "project")
    if not project_id or project_id == "(unset)":
        raise RuntimeError("Select a GCP project first: gcloud config set project PROJECT_ID")

    resource_name = resolve("MOVIEREC_RESOURCE_NAME", file_env, process_env, tfvars)
    if not re.fullmatch(r"[a-z]([-a-z0-9]*[a-z0-9])?", resource_name):
        raise RuntimeError(
            "Set MOVIEREC_RESOURCE_NAME in deploy/gcp/secrets/server.env or its example; "
            "it must be a lowercase GCP resource name."
        )
    active_zone = gcloud_value("config", "get-value", "compute/zone")
    configured_zone = first_value(
        file_env.get("GCP_ZONE"),
        process_env.get("GCP_ZONE"),
        file_env.get("TF_VAR_zone"),
        process_env.get("TF_VAR_zone"),
        tfvars.get("zone"),
    )

    try:
        listed = subprocess.run(
            [
                "gcloud", "compute", "instances", "list",
                f"--project={project_id}",
                f"--filter=name={resource_name}-app",
                "--format=value(zone.basename())",
                "--quiet",
            ],
            check=False,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    except FileNotFoundError as error:
        raise RuntimeError("gcloud is required; run this command from Google Cloud Shell.") from error
    if listed.returncode != 0:
        error_text = listed.stderr.strip()
        disabled_compute_api = (
            "compute.googleapis.com" in error_text.lower()
            and any(marker in error_text.lower() for marker in ("disabled", "not enabled", "not been used"))
        )
        if disabled_compute_api:
            print("Compute Engine API is not enabled yet; using the configured zone for a fresh deployment.")
            app_zones = []
        else:
            raise RuntimeError(f"Could not discover the app VM zone:\n{error_text}")
    else:
        app_zones = listed.stdout.splitlines()
    app_zones = [zone.strip() for zone in app_zones if zone.strip()]

    if len(app_zones) == 1:
        zone = app_zones[0]
        zone_source = f"the {resource_name}-app VM"
    elif len(app_zones) > 1:
        zone = next((candidate for candidate in (active_zone, configured_zone) if candidate in app_zones), "")
        if not zone:
            choices = ", ".join(app_zones)
            raise RuntimeError(
                f"Found {resource_name}-app VMs in multiple zones ({choices}); "
                "set gcloud compute/zone to one of them and rerun make glean-vars."
            )
        zone_source = f"the configured zone matching a {resource_name}-app VM"
    else:
        zone = first_value(
            active_zone if active_zone != "(unset)" else "",
            configured_zone,
        )
        zone_source = "gcloud compute/zone" if zone == active_zone else "the existing zone setting"

    if not zone or zone == "(unset)" or "-" not in zone:
        raise RuntimeError(
            "Could not determine GCP_ZONE. Set it with "
            "gcloud config set compute/zone ZONE, then rerun make glean-vars."
        )

    region = zone.rsplit("-", 1)[0]
    model_instance: dict[str, object] | None = None
    model_settings: dict[str, object] | None = None
    if listed.returncode == 0:
        model_name = f"{resource_name}-model"
        model_instances = gcloud_instances(project_id, model_name)
        if len(model_instances) == 1:
            model_instance = model_instances[0]
        elif len(model_instances) > 1:
            configured_model_zone = tfvars.get("model_zone", "")
            if configured_model_zone in {"", "null"}:
                configured_model_zone = ""
            candidates = (configured_model_zone, active_zone, zone)
            for candidate in candidates:
                if not candidate or candidate == "(unset)":
                    continue
                matching = [
                    instance
                    for instance in model_instances
                    if resource_basename(instance.get("zone")) == candidate
                ]
                if len(matching) == 1:
                    model_instance = matching[0]
                    break
            if model_instance is None:
                model_zones = sorted(
                    {resource_basename(instance.get("zone")) for instance in model_instances}
                )
                raise RuntimeError(
                    f"Found multiple {model_name} VMs in zones {', '.join(model_zones)}; "
                    "set model_zone in terraform.tfvars or gcloud compute/zone before gleaning."
                )

        if model_instance is not None:
            try:
                model_settings = model_gpu_settings(model_instance)
            except (TypeError, ValueError) as error:
                print(f"Could not glean model GPU settings: {error}", file=sys.stderr)

    artifact_region = tfvars.get("artifact_registry_region", "")
    if artifact_region in {"", "null"}:
        artifact_region = first_value(
            file_env.get("GCP_ARTIFACT_REGISTRY_REGION"),
            process_env.get("GCP_ARTIFACT_REGISTRY_REGION"),
        )
    for name, value in (
        ("GCP_PROJECT_ID", project_id),
        ("GCP_REGION", region),
        ("GCP_ZONE", zone),
        ("GCP_ARTIFACT_REGISTRY_REGION", artifact_region),
    ):
        if any(char.isspace() or char in "#$=," for char in value):
            raise RuntimeError(f"{name} contains characters that cannot be imported by Make.")

    write_tfvar("project_id", project_id)
    write_tfvar("region", region)
    write_tfvar("zone", zone)
    write_tfvar("name", resource_name)
    if model_instance is not None:
        model_zone = resource_basename(model_instance.get("zone"))
        write_tfvar("model_zone", model_zone)
        if model_settings is not None:
            gpu_enabled = bool(model_settings["enabled"])
            write_tfvar("model_gpu_enabled", str(gpu_enabled).lower(), raw=True)
            if gpu_enabled:
                gpu_type = str(model_settings["gpu_type"])
                gpu_machine_type = str(model_settings["machine_type"])
                gpu_count = int(model_settings["gpu_count"])
                write_tfvar("model_gpu_type", gpu_type)
                write_tfvar("model_gpu_machine_type", gpu_machine_type)
                write_tfvar("model_gpu_count", str(gpu_count), raw=True)
                write_server_env("GPU_TYPE", gpu_type)
                write_server_env("GPU_MACHINE_TYPE", gpu_machine_type)
                write_server_env("GPU_COUNT", str(gpu_count))
                print(
                    f"Gleaned model GPU {gpu_type} ({gpu_machine_type}, count {gpu_count}) "
                    f"from the {model_zone} VM."
                )
            else:
                write_tfvar("model_cpu_machine_type", str(model_settings["machine_type"]))
                print(f"Gleaned CPU model VM ({model_settings['machine_type']}) in {model_zone}.")

    write_server_env("GCP_PROJECT_ID", project_id)
    write_server_env("GCP_REGION", region)
    write_server_env("GCP_ZONE", zone)
    if artifact_region:
        write_server_env("GCP_ARTIFACT_REGISTRY_REGION", artifact_region)
    write_server_env("MOVIEREC_RESOURCE_NAME", resource_name)
    for legacy_name in ("TF_VAR_project_id", "TF_VAR_region", "TF_VAR_zone", "TF_VAR_name"):
        remove_server_env(legacy_name)
    os.chmod(SERVER_ENV, 0o600)

    print(f"Saved GCP project {project_id}, zone {zone}, and region {region} from {zone_source}.")
    print(f"Saved Make and Terraform settings in {SERVER_ENV} and {TFVARS}.")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)

    commands.add_parser("glean-vars", help="discover GCP settings and save them for Make and Terraform")
    commands.add_parser("settings", help="print resolved deployment settings for GCP helper scripts")

    set_parser = commands.add_parser("set-tfvar", help="set a Terraform variable")
    set_parser.add_argument("key")
    set_parser.add_argument("value")
    set_parser.add_argument("--raw", action="store_true", help="write value as a Terraform expression")

    get_parser = commands.add_parser("get-tfvar", help="read a Terraform variable")
    get_parser.add_argument("key")

    set_env_parser = commands.add_parser("set-server-env", help="set a value in the server environment file")
    set_env_parser.add_argument("key")
    set_env_parser.add_argument("value")

    gpu_parser = commands.add_parser("gpu-mode", help="validate GPU options and print the selected mode for Make")
    gpu_parser.add_argument("--mode", required=True)
    gpu_parser.add_argument("--type", required=True)
    gpu_parser.add_argument("--machine-type", required=True)
    gpu_parser.add_argument("--count", type=int, required=True)

    set_gpu_parser = commands.add_parser("set-gpu-vars", help="persist an explicit GPU choice in terraform.tfvars")
    set_gpu_parser.add_argument("--mode", required=True)
    set_gpu_parser.add_argument("--type", required=True)
    set_gpu_parser.add_argument("--machine-type", required=True)
    set_gpu_parser.add_argument("--count", type=int, required=True)

    terraform_flags_parser = commands.add_parser(
        "terraform-flags", help="validate Make's Terraform switches and print operation flags"
    )
    terraform_flags_parser.add_argument("--operation", choices=("plan", "apply"), required=True)
    terraform_flags_parser.add_argument("--model-only", required=True)
    terraform_flags_parser.add_argument("--auto-approve", required=True)

    args = parser.parse_args()
    if args.command == "glean-vars":
        try:
            glean_vars()
        except (OSError, RuntimeError, ValueError) as error:
            parser.error(str(error))
    elif args.command == "settings":
        file_env = read_server_env()
        process_env = dict(os.environ)
        tfvars = read_tfvars({"region", "zone", "artifact_registry_region", "name"})
        names = (
            "GCP_PROJECT_ID",
            "GCP_REGION",
            "GCP_ZONE",
            "GCP_ARTIFACT_REGISTRY_REGION",
            "MOVIEREC_RESOURCE_NAME",
        )
        for name in names:
            value = resolve(name, file_env, process_env, tfvars)
            if any(char.isspace() or char in "#$=," for char in value):
                raise ValueError(f"{name} contains characters that cannot be represented in helper settings")
            print(f"{name}={value}")
    elif args.command == "set-tfvar":
        write_tfvar(args.key, args.value, raw=args.raw)
    elif args.command == "get-tfvar":
        print(read_tfvars({args.key}).get(args.key, ""))
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
    elif args.command == "terraform-flags":
        try:
            print(terraform_flags(args.operation, args.model_only, args.auto_approve))
        except ValueError as error:
            parser.error(str(error))
    elif args.command == "set-server-env":
        write_server_env(args.key, args.value)


if __name__ == "__main__":
    main()
