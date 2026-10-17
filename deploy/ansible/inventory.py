#!/usr/bin/env python3
"""Dynamic Ansible inventory built from the provisioned GCP instances.

The VMs have no public SSH access, so every connection goes through an IAP
tunnel using the key that `gcloud compute ssh` manages.
"""
import getpass
import json
import os
import subprocess
import sys


def active_project():
    project_id = os.environ.get("GCP_PROJECT_ID") or os.environ.get("TF_VAR_project_id")
    if project_id:
        return project_id

    result = subprocess.run(
        ["gcloud", "config", "get-value", "project"],
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        sys.exit(f"Could not determine the GCP project ID:\n{result.stderr}")
    project_id = result.stdout.strip()
    if not project_id or project_id == "(unset)":
        sys.exit("Set GCP_PROJECT_ID or select an active project with gcloud config set project.")
    return project_id


def deployment_config():
    project_id = active_project()
    zone = os.environ.get("GCP_ZONE", "")
    region = os.environ.get("GCP_REGION", "")
    artifact_registry_region = os.environ.get("GCP_ARTIFACT_REGISTRY_REGION") or region
    resource_name = os.environ.get("MOVIEREC_RESOURCE_NAME", "")
    if not zone or not region or not resource_name:
        sys.exit("GCP_ZONE, GCP_REGION, and MOVIEREC_RESOURCE_NAME must not be empty.")

    def describe_instance(role, instance_zone):
        name = f"{resource_name}-{role}"
        result = subprocess.run(
            [
                "gcloud", "compute", "instances", "describe", name,
                f"--project={project_id}", f"--zone={instance_zone}", "--format=json", "--quiet",
            ],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            sys.exit(
                f"Could not describe GCP instance {name} in {instance_zone}:\n{result.stderr}"
            )
        try:
            return json.loads(result.stdout)
        except json.JSONDecodeError as error:
            sys.exit(f"gcloud returned invalid instance data for {name}: {error}")

    model_name = f"{resource_name}-model"
    model_locations = subprocess.run(
        [
            "gcloud", "compute", "instances", "list",
            f"--project={project_id}", f"--filter=name={model_name}",
            "--format=value(zone)", "--quiet",
        ],
        capture_output=True,
        text=True,
    )
    if model_locations.returncode != 0:
        sys.exit(f"Could not find the model VM zone:\n{model_locations.stderr}")
    model_zones = [
        location.strip().rsplit("/", 1)[-1]
        for location in model_locations.stdout.splitlines()
        if location.strip()
    ]
    if len(model_zones) != 1:
        sys.exit(
            f"Expected one GCP VM named {model_name}, found {len(model_zones)}. "
            "Resolve duplicate or missing model VMs before running Ansible."
        )

    zones = {"app": zone, "database": zone, "model": model_zones[0]}
    instances = {
        role: describe_instance(role, zones[role])
        for role in ("app", "database", "model")
    }
    app = instances["app"]
    interfaces = app.get("networkInterfaces", [])
    if not interfaces or not interfaces[0].get("networkIP"):
        sys.exit(f"GCP instance {app.get('name', 'app')} has no private network IP.")
    public_ip = next(
        (
            config.get("natIP")
            for interface in interfaces
            for config in interface.get("accessConfigs", [])
            if config.get("natIP")
        ),
        None,
    )
    if not public_ip:
        sys.exit(f"GCP instance {app.get('name', 'app')} has no public NAT IP.")

    model_metadata = {
        item["key"]: item.get("value", "")
        for item in instances["model"].get("metadata", {}).get("items", [])
        if "key" in item
    }
    gpu_enabled = model_metadata.get(
        "gpu_enabled", os.environ.get("MOVIEREC_MODEL_GPU_ENABLED", "")
    ).lower() == "true"

    return {
        "project_id": project_id,
        "region": region,
        "zone": zone,
        "model_zone": zones["model"],
        "app_instance_name": instances["app"]["name"],
        "database_instance_name": instances["database"]["name"],
        "model_instance_name": instances["model"]["name"],
        "app_private_ip": interfaces[0]["networkIP"],
        "public_ip": public_ip,
        "artifact_registry": f"{artifact_registry_region}-docker.pkg.dev/{project_id}/{resource_name}",
        "model_gpu_enabled": gpu_enabled,
    }


def build_inventory():
    out = deployment_config()
    ssh_user = os.environ.get("MOVIEREC_SSH_USER") or getpass.getuser()
    key_file = os.path.expanduser("~/.ssh/google_compute_engine")

    roles = {
        "app": out["app_instance_name"],
        "database": out["database_instance_name"],
        "model": out["model_instance_name"],
    }
    hostvars = {
        name: {
            "ansible_host": name,
            "node_role": role,
            "node_label": f"movierec.io/role={role}",
            "ansible_ssh_common_args": (
                "-o ProxyCommand='gcloud compute start-iap-tunnel %h 22 "
                f"--project={out['project_id']} --listen-on-stdin "
                f"--zone={out['model_zone'] if role == 'model' else out['zone']} "
                "--verbosity=warning' "
                "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o CheckHostIP=no"
            ),
        }
        for role, name in roles.items()
    }
    return {
        "_meta": {"hostvars": hostvars},
        "all": {
            "vars": {
                "ansible_user": ssh_user,
                "ansible_ssh_private_key_file": key_file,
                "app_private_ip": out["app_private_ip"],
                "public_ip": out["public_ip"],
                "artifact_registry": out["artifact_registry"],
                "model_gpu_enabled": out["model_gpu_enabled"],
            }
        },
        **{role: {"hosts": [name]} for role, name in roles.items()},
    }


def main():
    if len(sys.argv) == 2 and sys.argv[1] == "--list":
        print(json.dumps(build_inventory()))
    elif len(sys.argv) == 3 and sys.argv[1] == "--host":
        print(json.dumps({}))
    else:
        sys.exit("usage: inventory.py --list | --host NAME")


if __name__ == "__main__":
    main()
