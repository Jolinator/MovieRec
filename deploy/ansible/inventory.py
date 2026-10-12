#!/usr/bin/env python3
"""Dynamic Ansible inventory built from the Terraform outputs.

The VMs have no public SSH access, so every connection goes through an IAP
tunnel using the key that `gcloud compute ssh` manages.
"""
import getpass
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
TF_DIR = os.path.join(HERE, "..", "gcp", "terraform")


def terraform_outputs():
    result = subprocess.run(
        ["terraform", f"-chdir={TF_DIR}", "output", "-json"],
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        sys.exit(f"terraform output failed (run make infra-apply first):\n{result.stderr}")
    outputs = {name: value["value"] for name, value in json.loads(result.stdout).items()}
    required = [
        "zone", "app_instance_name", "database_instance_name", "model_instance_name",
        "app_private_ip", "public_ip", "artifact_registry", "model_gpu_enabled",
    ]
    missing = [name for name in required if name not in outputs]
    if missing:
        sys.exit(f"Terraform outputs are incomplete: {', '.join(missing)}")
    return outputs


def build_inventory():
    out = terraform_outputs()
    iap_proxy = (
        "-o ProxyCommand='gcloud compute start-iap-tunnel %h 22 "
        f"--listen-on-stdin --zone={out['zone']} --verbosity=warning' "
        "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o CheckHostIP=no"
    )
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
        }
        for role, name in roles.items()
    }
    return {
        "_meta": {"hostvars": hostvars},
        "all": {
            "vars": {
                "ansible_user": ssh_user,
                "ansible_ssh_private_key_file": key_file,
                "ansible_ssh_common_args": iap_proxy,
                "app_private_ip": out["app_private_ip"],
                "public_ip": out["public_ip"],
                "artifact_registry": out["artifact_registry"],
                "terraform_gpu_enabled": out["model_gpu_enabled"],
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
