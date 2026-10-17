#!/usr/bin/env python3
"""Move selected MovieRec VMs to a target zone, then restart and redeploy."""

from __future__ import annotations

import argparse
import datetime as dt
import ipaddress
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
GCP_CONFIG = REPO_ROOT / "scripts/gcp/gcp-config.py"
SERVER_ENV = REPO_ROOT / "deploy/gcp/secrets/server.env"
TFVARS = REPO_ROOT / "deploy/gcp/terraform/terraform.tfvars"
TF_DIR = REPO_ROOT / "deploy/gcp/terraform"
TF_REPOSITORY_ADDRESS = "google_artifact_registry_repository.images"
TF_PUBLIC_READER_PREFIX = "google_artifact_registry_repository_iam_member.public_reader["


def run(command: list[str], *, capture: bool = False) -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(
            command,
            check=True,
            text=True,
            stdout=subprocess.PIPE if capture else None,
            stderr=subprocess.PIPE if capture else None,
        )
    except subprocess.CalledProcessError as error:
        if capture:
            if error.stdout:
                print(error.stdout, end="")
            if error.stderr:
                print(error.stderr, end="", file=sys.stderr)
        raise


def deployment_settings() -> dict[str, str]:
    result = run([sys.executable, str(GCP_CONFIG), "settings"], capture=True)
    return dict(line.split("=", 1) for line in result.stdout.splitlines() if "=" in line)


def terraform_var(key: str) -> str:
    return run([sys.executable, str(GCP_CONFIG), "get-tfvar", key], capture=True).stdout.strip()


def terraform_state_addresses() -> set[str]:
    result = run(["terraform", f"-chdir={TF_DIR}", "state", "list"], capture=True)
    return {line.strip() for line in result.stdout.splitlines() if line.strip()}


def ensure_destination_registry(
    *, project: str, repository: str, location: str, public_reader: bool
) -> None:
    listed = run(
        [
            "gcloud", "artifacts", "repositories", "list",
            f"--project={project}", f"--location={location}", "--format=json", "--quiet",
        ],
        capture=True,
    )
    repositories = json.loads(listed.stdout)
    existing = next(
        (
            item for item in repositories
            if item.get("name", "").rstrip("/").rsplit("/", 1)[-1] == repository
        ),
        None,
    )
    if existing is None:
        run(
            [
                "gcloud", "artifacts", "repositories", "create", repository,
                f"--project={project}", f"--location={location}",
                "--repository-format=docker", "--description=MovieRec container images", "--quiet",
            ]
        )
        print(f"Created destination Artifact Registry: {location}/{repository}")
    elif existing.get("format") != "DOCKER":
        raise ValueError(
            f"Destination repository {location}/{repository} exists with format "
            f"{existing.get('format')!r}, expected 'DOCKER'."
        )
    else:
        print(f"Using existing destination Artifact Registry: {location}/{repository}")

    if public_reader:
        run(
            [
                "gcloud", "artifacts", "repositories", "add-iam-policy-binding", repository,
                f"--project={project}", f"--location={location}",
                "--member=allUsers", "--role=roles/artifactregistry.reader", "--quiet",
            ]
        )


def registry_locations(project: str, repository: str) -> set[str]:
    listed = run(
        [
            "gcloud", "artifacts", "repositories", "list",
            f"--project={project}", "--location=all", "--format=json", "--quiet",
        ],
        capture=True,
    )
    repositories = json.loads(listed.stdout)
    locations = set()
    for item in repositories:
        name = item.get("name", "").rstrip("/")
        match = re.fullmatch(r"projects/[^/]+/locations/([^/]+)/repositories/([^/]+)", name)
        if match and match.group(2) == repository and item.get("format") == "DOCKER":
            locations.add(match.group(1))
    return locations


def import_registry_state(project: str, location: str, repository: str, public_reader: bool) -> None:
    repo_id = f"projects/{project}/locations/{location}/repositories/{repository}"
    run(["terraform", f"-chdir={TF_DIR}", "import", TF_REPOSITORY_ADDRESS, repo_id])
    if public_reader:
        run(
            [
                "terraform", f"-chdir={TF_DIR}", "import",
                f'{TF_PUBLIC_READER_PREFIX}"allUsers"]',
                f"{repo_id} roles/artifactregistry.reader allUsers",
            ]
        )


def remap_registry_in_terraform(
    *, project: str, repository: str, old_location: str, new_location: str,
    public_reader: bool,
) -> None:
    addresses = terraform_state_addresses()
    if TF_REPOSITORY_ADDRESS not in addresses:
        raise ValueError(
            f"Terraform state does not contain {TF_REPOSITORY_ADDRESS}; "
            "refusing to delete the old repository without managed destination state."
        )
    old_reader_addresses = sorted(
        address for address in addresses if address.startswith(TF_PUBLIC_READER_PREFIX)
    )

    run([sys.executable, str(GCP_CONFIG), "set-tfvar", "artifact_registry_region", new_location])
    run([sys.executable, str(GCP_CONFIG), "set-server-env", "GCP_ARTIFACT_REGISTRY_REGION", new_location])
    try:
        if old_reader_addresses:
            run(["terraform", f"-chdir={TF_DIR}", "state", "rm", *old_reader_addresses])
        run(["terraform", f"-chdir={TF_DIR}", "state", "rm", TF_REPOSITORY_ADDRESS])
        import_registry_state(project, new_location, repository, public_reader)
    except (subprocess.CalledProcessError, OSError, ValueError) as error:
        print(
            f"Terraform could not adopt the new registry ({error}); restoring its old registry state.",
            file=sys.stderr,
        )
        try:
            current_addresses = terraform_state_addresses()
            current_readers = sorted(
                address for address in current_addresses
                if address.startswith(TF_PUBLIC_READER_PREFIX)
            )
            if current_readers:
                run(["terraform", f"-chdir={TF_DIR}", "state", "rm", *current_readers])
            if TF_REPOSITORY_ADDRESS in current_addresses:
                run(["terraform", f"-chdir={TF_DIR}", "state", "rm", TF_REPOSITORY_ADDRESS])
            run([sys.executable, str(GCP_CONFIG), "set-tfvar", "artifact_registry_region", old_location])
            run([sys.executable, str(GCP_CONFIG), "set-server-env", "GCP_ARTIFACT_REGISTRY_REGION", old_location])
            import_registry_state(project, old_location, repository, bool(old_reader_addresses))
        except (subprocess.CalledProcessError, OSError, ValueError) as rollback_error:
            print(
                "Automatic registry-state recovery also failed. Both repositories were retained; "
                f"inspect Terraform state before applying again: {rollback_error}",
                file=sys.stderr,
            )
        raise



def delete_registries_outside_target(project: str, repository: str, target_location: str) -> None:
    listed = run(
        [
            "gcloud", "artifacts", "repositories", "list",
            f"--project={project}", "--location=all", "--format=json", "--quiet",
        ],
        capture=True,
    )
    repositories = json.loads(listed.stdout)
    old_locations = []
    for item in repositories:
        name = item.get("name", "").rstrip("/")
        match = re.fullmatch(r"projects/[^/]+/locations/([^/]+)/repositories/([^/]+)", name)
        if not match or match.group(2) != repository or item.get("format") != "DOCKER":
            continue
        location = match.group(1)
        if location != target_location:
            old_locations.append(location)

    for location in sorted(set(old_locations)):
        run(
            [
                "gcloud", "artifacts", "repositories", "delete", repository,
                f"--project={project}", f"--location={location}", "--quiet",
            ]
        )
        print(f"Deleted previous Artifact Registry: {location}/{repository}")
    if not old_locations:
        print(f"No other {repository} Docker repositories found outside {target_location}.")


def region_of(zone: str) -> str:
    if not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)+-[a-z]", zone):
        raise ValueError(f"Invalid Google Cloud zone: {zone}")
    return zone.rsplit("-", 1)[0]


def describe_instance(name: str, zone: str, project: str) -> dict:
    result = run(
        [
            "gcloud", "compute", "instances", "describe", name,
            f"--project={project}", f"--zone={zone}", "--format=json", "--quiet",
        ],
        capture=True,
    )
    return json.loads(result.stdout)


def disk_snapshot_source(instance: dict, role: str) -> list[tuple[str, str]]:
    disks = instance.get("disks", [])
    boot_disks = [disk for disk in disks if disk.get("boot")]
    data_disks = [disk for disk in disks if not disk.get("boot") and disk.get("type") != "SCRATCH"]
    if len(boot_disks) != 1 or len(data_disks) != 1:
        raise ValueError(
            f"Expected one boot and one persistent data disk on {role}; "
            f"found {len(boot_disks)} boot and {len(data_disks)} data disks."
        )

    sources = []
    for kind, disk in (("boot", boot_disks[0]), ("data", data_disks[0])):
        source = disk.get("source", "").rstrip("/").rsplit("/", 1)[-1]
        if not source:
            raise ValueError(f"Could not determine the {kind} disk name for {role}.")
        sources.append((kind, source))
    return sources


def snapshot_name(resource_name: str, role: str, kind: str, stamp: str) -> str:
    suffix = f"-{role}-{kind}-{stamp}"
    prefix = re.sub(r"[^a-z0-9-]", "-", resource_name.lower()).strip("-")
    return f"{prefix[: 63 - len(suffix)].rstrip('-')}{suffix}"


def snapshot_disks(
    instances: dict[str, dict], zones: dict[str, str], project: str, resource_name: str
) -> tuple[dict[str, str], list[str]]:
    stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dt%H%M%sz").lower()
    snapshot_vars: dict[str, str] = {}
    snapshot_names: list[str] = []
    for role in instances:
        for kind, disk_name in disk_snapshot_source(instances[role], role):
            name = snapshot_name(resource_name, role, kind, stamp)
            run(
                [
                    "gcloud", "compute", "disks", "snapshot", disk_name,
                    f"--project={project}", f"--zone={zones[role]}",
                    f"--snapshot-names={name}",
                    f"--description=Pre-migration snapshot for {resource_name} {role} {kind}",
                    "--quiet",
                ]
            )
            snapshot_vars[f"{role}_{kind}_disk_snapshot" if kind == "data" else f"{role}_boot_disk_snapshot"] = (
                f"global/snapshots/{name}"
            )
            snapshot_names.append(name)
            print(f"Snapshot ready: {name}")
    return snapshot_vars, snapshot_names


def start_original_instances(
    roles: list[str], zones: dict[str, str], project: str, resource_name: str
) -> None:
    for role in roles:
        name = f"{resource_name}-{role}"
        try:
            run(
                [
                    "gcloud", "compute", "instances", "start", name,
                    f"--project={project}", f"--zone={zones[role]}", "--quiet",
                ]
            )
        except (subprocess.CalledProcessError, OSError) as error:
            print(f"Could not restart the original {role} VM: {error}", file=sys.stderr)


def restore_local_config(tfvars_backup: bytes, server_env_backup: bytes | None) -> None:
    TFVARS.write_bytes(tfvars_backup)
    if server_env_backup is None:
        SERVER_ENV.unlink(missing_ok=True)
    else:
        SERVER_ENV.write_bytes(server_env_backup)


def update_migration_config(
    *, target_zone: str, target_model_zone: str, target_region: str, old_region: str,
    app_private_ip: str, snapshots: dict[str, str], artifact_region: str,
) -> None:
    values = {
        "region": target_region,
        "zone": target_zone,
        "model_zone": target_model_zone,
        "artifact_registry_region": artifact_region or old_region,
        "app_private_ip": app_private_ip,
        **snapshots,
    }
    for key, value in values.items():
        run([sys.executable, str(GCP_CONFIG), "set-tfvar", key, value])
    run([sys.executable, str(GCP_CONFIG), "set-server-env", "GCP_REGION", target_region])
    run([sys.executable, str(GCP_CONFIG), "set-server-env", "GCP_ZONE", target_zone])
    run([
        sys.executable, str(GCP_CONFIG), "set-server-env",
        "GCP_ARTIFACT_REGISTRY_REGION", artifact_region or old_region,
    ])


def instance_zone(name: str, project: str) -> str:
    result = run(
        [
            "gcloud", "compute", "instances", "list", f"--project={project}",
            f"--filter=name={name}", "--format=json", "--quiet",
        ],
        capture=True,
    )
    instances = json.loads(result.stdout)
    matches = [item for item in instances if item.get("name") == name]
    if len(matches) != 1:
        raise ValueError(f"Expected one VM named {name}, found {len(matches)}.")
    zone = matches[0].get("zone", "").rstrip("/").rsplit("/", 1)[-1]
    if not zone:
        raise ValueError(f"Could not determine the zone for VM {name}.")
    return zone


def stop_instances(roles: list[str], zones: dict[str, str], project: str, resource_name: str) -> None:
    by_zone: dict[str, list[str]] = {}
    for role in roles:
        by_zone.setdefault(zones[role], []).append(f"{resource_name}-{role}")
    for zone, names in by_zone.items():
        run(
            [
                "gcloud", "compute", "instances", "stop", *names,
                f"--project={project}", f"--zone={zone}", "--quiet",
            ]
        )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--old-zone", required=True, help="Current app/database zone")
    parser.add_argument(
        "--target-zone", "--target-model-zone", dest="target_zone", required=True,
        help="Destination zone for the VMs being moved",
    )
    parser.add_argument(
        "--scope", choices=("app-db", "all"), required=True,
        help="Move only app/database, or the full cluster including the model VM",
    )
    parser.add_argument("--image-tag", help="Optional tag for the image build and deployment")
    parser.add_argument("--public-host", help="Optional custom hostname for the deployment")
    args = parser.parse_args()

    for command in ("gcloud", "make", "terraform", "docker", "ansible-playbook"):
        if shutil.which(command) is None:
            parser.error(f"Required command not found: {command}")
    if args.image_tag and not re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}", args.image_tag):
        parser.error("IMAGE_TAG contains characters that are not valid for a Docker tag.")

    try:
        old_region = region_of(args.old_zone)
        target_region = region_of(args.target_zone)
    except ValueError as error:
        parser.error(str(error))

    if not TFVARS.is_file() or not SERVER_ENV.is_file():
        parser.error("Initialize configuration first with make secrets-init.")

    settings = deployment_settings()
    configured_zone = settings.get("GCP_ZONE", "")
    configured_region = settings.get("GCP_REGION", "")
    if configured_zone != args.old_zone:
        parser.error(
            f"OLD_ZONE is {args.old_zone}, but Terraform currently configures {configured_zone!r}. "
            "Use the current app/database zone from terraform.tfvars."
        )
    if configured_region != old_region:
        parser.error(
            f"The configured region {configured_region!r} does not match OLD_ZONE {args.old_zone}."
        )

    project = settings.get("GCP_PROJECT_ID") or settings.get("TF_VAR_project_id") or os.environ.get("GCP_PROJECT_ID", "")
    if not project:
        project = run(["bash", str(REPO_ROOT / "scripts/gcp/gcp-context.sh"), "project-id"], capture=True).stdout.strip()
    resource_name = settings.get("MOVIEREC_RESOURCE_NAME", "")
    if not project:
        parser.error("Set GCP_PROJECT_ID or select the correct project with gcloud config set project.")
    if not resource_name:
        parser.error("Set MOVIEREC_RESOURCE_NAME in deploy/gcp/secrets/server.env.")

    try:
        run(["terraform", f"-chdir={TF_DIR}", "init"])
        tf_addresses = terraform_state_addresses()
    except (subprocess.CalledProcessError, OSError) as error:
        parser.error(f"Could not initialize or read Terraform state before migration: {error}")
    if TF_REPOSITORY_ADDRESS not in tf_addresses:
        parser.error(
            f"Terraform state does not manage {TF_REPOSITORY_ADDRESS}; "
            "cannot safely switch and clean up Artifact Registry."
        )

    try:
        model_zone = instance_zone(f"{resource_name}-model", project)
        roles_to_move = ["app", "database"]
        if args.scope == "all" and model_zone != args.target_zone:
            roles_to_move.append("model")
        zones = {"app": args.old_zone, "database": args.old_zone, "model": model_zone}
        instances = {
            role: describe_instance(f"{resource_name}-{role}", zones[role], project)
            for role in roles_to_move
        }
        if args.scope == "all" and model_zone == args.target_zone:
            print(f"Model VM is already in {args.target_zone}; leaving it in place.")
    except (subprocess.CalledProcessError, json.JSONDecodeError, ValueError) as error:
        parser.error(f"Could not verify the source VMs: {error}")

    if args.old_zone == args.target_zone and "model" not in roles_to_move:
        parser.error("The app/database VMs are already in the target zone; no migration is needed.")
    target_model_zone = args.target_zone if args.scope == "all" else model_zone

    app_interfaces = instances["app"].get("networkInterfaces", [])
    app_private_ip = app_interfaces[0].get("networkIP", "") if app_interfaces else ""
    try:
        address = ipaddress.IPv4Address(app_private_ip)
    except ipaddress.AddressValueError:
        parser.error(f"Could not read a valid private IP from the app VM: {app_private_ip!r}.")
    subnet = ipaddress.IPv4Network("10.20.0.0/24")
    if address not in subnet or int(address) <= int(subnet.network_address) + 1 or address == subnet.broadcast_address:
        parser.error(
            f"App private IP {address} is outside the reusable host range for the configured 10.20.0.0/24 subnet."
        )

    unsupported_states = {
        role: instance.get("status")
        for role, instance in instances.items()
        if instance.get("status") not in {"RUNNING", "TERMINATED"}
    }
    if unsupported_states:
        parser.error(f"VMs being moved must be RUNNING or TERMINATED: {unsupported_states}")

    initially_running = [role for role, instance in instances.items() if instance.get("status") == "RUNNING"]
    artifact_region = settings.get("GCP_ARTIFACT_REGISTRY_REGION") or old_region
    old_reader_addresses = [
        address for address in tf_addresses if address.startswith(TF_PUBLIC_READER_PREFIX)
    ]
    public_reader = terraform_var("public_registry").lower() == "true" or bool(old_reader_addresses)
    try:
        existing_registry_locations = registry_locations(project, resource_name)
    except (subprocess.CalledProcessError, OSError, ValueError, json.JSONDecodeError) as error:
        parser.error(f"Could not list the app's Artifact Registry repositories: {error}")
    cleanup_locations = sorted(existing_registry_locations - {target_region})
    print(f"Project: {project}")
    print(f"App/database: {args.old_zone} -> {args.target_zone}")
    if "model" in roles_to_move:
        print(f"Model: {model_zone} -> {args.target_zone}")
    else:
        print(f"Model: stays in {model_zone}")
    print(f"Preserved app private IP: {app_private_ip}")
    if artifact_region == target_region:
        print(f"Artifact Registry already uses target region: {target_region}")
    else:
        print(
            f"Artifact Registry: {artifact_region} -> {target_region}"
        )
    print(
        "MovieRec Docker repository copies to remove after successful deployment: "
        + (", ".join(cleanup_locations) if cleanup_locations else "none")
    )
    print("This causes downtime. The app's public IP will change; update any DNS record after migration.")
    moving_roles = ", ".join(roles_to_move)
    snapshot_count = 2 * len(roles_to_move)
    confirmation = input(
        f"Type MIGRATE to stop {moving_roles}, create {snapshot_count} disk snapshots, "
        "and proceed with the listed post-deploy cleanup: "
    ).strip()
    if confirmation != "MIGRATE":
        print("Cancelled.")
        return 0

    stopped_roles: list[str] = []
    try:
        if initially_running:
            stopped_roles = initially_running.copy()
            stop_instances(stopped_roles, zones, project, resource_name)
        snapshot_vars, snapshot_names = snapshot_disks(instances, zones, project, resource_name)
    except (subprocess.CalledProcessError, OSError, ValueError) as error:
        print(f"Snapshot preparation failed: {error}", file=sys.stderr)
        start_original_instances(stopped_roles, zones, project, resource_name)
        return 1

    tfvars_backup = TFVARS.read_bytes()
    server_env_backup = SERVER_ENV.read_bytes() if SERVER_ENV.exists() else None
    try:
        update_migration_config(
            target_zone=args.target_zone,
            target_model_zone=target_model_zone,
            target_region=target_region,
            old_region=old_region,
            app_private_ip=str(address),
            snapshots=snapshot_vars,
            artifact_region=artifact_region,
        )
        run(["make", "infra-plan", "GPU=auto"])
    except (subprocess.CalledProcessError, OSError, ValueError) as error:
        print(f"Migration plan failed; restoring the previous local config: {error}", file=sys.stderr)
        restore_local_config(tfvars_backup, server_env_backup)
        start_original_instances(stopped_roles, zones, project, resource_name)
        print(f"Snapshots were retained for recovery: {', '.join(snapshot_names)}")
        return 1

    confirmation = input(
        f"Review the Terraform plan above. Type APPLY to recreate {moving_roles} in the target zone: "
    ).strip()
    if confirmation != "APPLY":
        restore_local_config(tfvars_backup, server_env_backup)
        start_original_instances(stopped_roles, zones, project, resource_name)
        print("Migration cancelled; original configuration restored and previously running VMs restarted.")
        print(f"Snapshots were retained: {', '.join(snapshot_names)}")
        return 0

    try:
        run(["make", "infra-apply", "GPU=auto", "TF_AUTO_APPROVE=1"])
    except (subprocess.CalledProcessError, OSError) as error:
        print(
            "Terraform apply did not complete. The migration config and snapshots were retained; "
            "inspect make infra-plan before retrying.",
            file=sys.stderr,
        )
        print(f"Apply error: {error}", file=sys.stderr)
        print(f"Snapshots: {', '.join(snapshot_names)}")
        return 1

    try:
        run(["make", "start"])
    except (subprocess.CalledProcessError, OSError) as error:
        print("The VMs were moved, but make start did not bring the cluster online.", file=sys.stderr)
        print(f"Start error: {error}", file=sys.stderr)
        return 1

    try:
        for role in roles_to_move:
            run(["make", "reboot", f"MACHINE={role}"])
    except (subprocess.CalledProcessError, OSError) as error:
        print("The VMs moved, but the requested restart sequence did not finish.", file=sys.stderr)
        print(f"Restart error: {error}", file=sys.stderr)
        return 1

    public_ip = ""
    try:
        migrated_app = describe_instance(f"{resource_name}-app", args.target_zone, project)
        interfaces = migrated_app.get("networkInterfaces", [])
        access_configs = interfaces[0].get("accessConfigs", []) if interfaces else []
        public_ip = access_configs[0].get("natIP", "") if access_configs else ""
    except (subprocess.CalledProcessError, json.JSONDecodeError):
        pass

    print("Infrastructure migration applied.")
    if public_ip:
        print(f"New app public IP: {public_ip}")
        print("Update your DNS record to point to this address if you use a custom hostname.")
    print(f"Recovery snapshots retained: {', '.join(snapshot_names)}")
    try:
        run(["make", "cluster-bootstrap", "GPU=auto"])
    except (subprocess.CalledProcessError, OSError) as error:
        print(
            "Infrastructure moved, but cluster reconnection did not finish. "
            "Run make cluster-bootstrap GPU=auto after resolving the reported issue.",
            file=sys.stderr,
        )
        print(f"Cluster bootstrap error: {error}", file=sys.stderr)
        return 1
    image_tag = args.image_tag or dt.datetime.now(dt.timezone.utc).strftime("migration-%Y%m%d%H%M%S")
    try:
        ensure_destination_registry(
            project=project,
            repository=resource_name,
            location=target_region,
            public_reader=public_reader,
        )
    except (subprocess.CalledProcessError, OSError, ValueError, json.JSONDecodeError) as error:
        print(
            "The VMs moved and K3s reconnected, but the destination Artifact Registry "
            f"could not be prepared. The previous repositories were retained: {error}",
            file=sys.stderr,
        )
        return 1
    print(f"Building and deploying MovieRec with image tag {image_tag}.")
    deploy_command = ["make", "update-app", f"IMAGE_TAG={image_tag}", "GPU=auto"]
    deploy_command.append(f"GCP_ARTIFACT_REGISTRY_REGION={target_region}")
    if args.public_host:
        deploy_command.append(f"PUBLIC_HOST={args.public_host}")
    try:
        run(deploy_command)
    except (subprocess.CalledProcessError, OSError) as error:
        print(
            "The VMs moved and the cluster reconnected, but the app build/deploy failed. "
            f"Retry with make update-app IMAGE_TAG={image_tag} GPU=auto.",
            file=sys.stderr,
        )
        print(f"Deploy error: {error}", file=sys.stderr)
        return 1
    if artifact_region != target_region:
        try:
            remap_registry_in_terraform(
                project=project,
                repository=resource_name,
                old_location=artifact_region,
                new_location=target_region,
                public_reader=public_reader,
            )
        except (subprocess.CalledProcessError, OSError, ValueError) as error:
            print(
                "Deployment succeeded, but Terraform registry cutover "
                f"did not complete: {error}",
                file=sys.stderr,
            )
            print(
                f"The new images are deployed from {target_region}; previous repositories were retained. "
                "Inspect Terraform state before retrying.",
                file=sys.stderr,
            )
            return 1
    try:
        delete_registries_outside_target(project, resource_name, target_region)
    except (subprocess.CalledProcessError, OSError, ValueError, json.JSONDecodeError) as error:
        print(
            f"Deployment succeeded, but cleanup of old Artifact Registry copies failed: {error}",
            file=sys.stderr,
        )
        return 1
    try:
        run(["make", "infra-apply", "GPU=auto", "TF_AUTO_APPROVE=1"])
    except (subprocess.CalledProcessError, OSError) as error:
        print(
            "The deployment succeeded and old repositories were deleted, but Terraform "
            f"could not reconcile the destination infrastructure: {error}",
            file=sys.stderr,
        )
        return 1
    print(f"Migration and deployment completed with IMAGE_TAG={image_tag}.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        print("Interrupted. Review the VM state and the migration snapshots before retrying.", file=sys.stderr)
        raise SystemExit(130)
