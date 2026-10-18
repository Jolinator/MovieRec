SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c

TF_DIR := deploy/gcp/terraform
GCP_SCRIPTS := scripts/gcp
GCP_CONFIG := python3 $(GCP_SCRIPTS)/gcp-config.py
SERVER_ENV := deploy/gcp/secrets/server.env
# The tracked example supplies defaults; glean-vars writes discovered values to server.env.
SERVER_ENV_EXAMPLE := deploy/gcp/secrets-examples/server.env.example
-include $(SERVER_ENV_EXAMPLE)
-include $(SERVER_ENV)

TF_PARALLELISM_FLAG = $(if $(strip $(TF_PARALLELISM)),-parallelism=$(TF_PARALLELISM),)
GPU_SET_VARS_ARGS = --mode "$(GPU)" --type "$(GPU_TYPE)" --machine-type "$(GPU_MACHINE_TYPE)" --count "$(GPU_COUNT)"
ifeq ($(origin TARGET_ZONE),undefined)
TARGET_ZONE := $(TARGET_MODEL_ZONE)
endif
TF_PLAN_FLAGS_RAW := $(shell $(GCP_CONFIG) terraform-flags --operation plan --model-only "$(MODEL_ONLY)" --auto-approve "$(TF_AUTO_APPROVE)" || echo invalid)
TF_APPLY_FLAGS_RAW := $(shell $(GCP_CONFIG) terraform-flags --operation apply --model-only "$(MODEL_ONLY)" --auto-approve "$(TF_AUTO_APPROVE)" || echo invalid)
ifneq ($(filter invalid,$(TF_PLAN_FLAGS_RAW) $(TF_APPLY_FLAGS_RAW)),)
$(error Invalid Terraform options; MODEL_ONLY and TF_AUTO_APPROVE must each be 0 or 1)
endif
TF_PLAN_FLAGS = $(if $(filter none,$(TF_PLAN_FLAGS_RAW)),,$(TF_PLAN_FLAGS_RAW))
TF_APPLY_FLAGS = $(if $(filter none,$(TF_APPLY_FLAGS_RAW)),,$(TF_APPLY_FLAGS_RAW))
GPU_CONFIG_MODE := $(shell $(GCP_CONFIG) gpu-mode --mode "$(GPU)" --type "$(GPU_TYPE)" --machine-type "$(GPU_MACHINE_TYPE)" --count "$(GPU_COUNT)")
ifneq ($(GPU_CONFIG_MODE),$(GPU))
$(error Invalid GPU settings; check GPU, GPU_TYPE, GPU_MACHINE_TYPE, and GPU_COUNT)
endif
ANSIBLE_DIR := deploy/ansible
ANSIBLE_CONFIG_FILE := $(CURDIR)/$(ANSIBLE_DIR)/ansible.cfg
ANSIBLE_PLAY := cd $(ANSIBLE_DIR) && ANSIBLE_CONFIG="$(ANSIBLE_CONFIG_FILE)" ansible-playbook -i inventory.py
SSH_ACCESS := bash $(GCP_SCRIPTS)/ssh-machine.sh app --command true
TF_PREPARE := terraform -chdir=$(TF_DIR) init -input=false && terraform -chdir=$(TF_DIR) fmt && terraform -chdir=$(TF_DIR) validate

export GCP_PROJECT_ID GCP_REGION GCP_ZONE GCP_ARTIFACT_REGISTRY_REGION MOVIEREC_RESOURCE_NAME NAMESPACE
export TF_PARALLELISM

.PHONY: help cloud-shell-setup glean-vars infra-plan infra-apply tf-backend-setup tf-backend-join \
        ssh-machine grafana grafana-stop pause-app resume-app reboot clear-databases clear-databases-reboot-all cluster-bootstrap secrets-init secrets-passwords set-region artifact-registry-cleanup docker-push-limit docker-push-limit-reset build-push \
        k8s-config models deploy update-app gpu-capacity container-logs start stop destroy benchmark benchmark-stress

help:
	@printf '%s\n' \
	  'Cloud workflow:' \
	  '  make cloud-shell-setup            Install tools, apply the configured Docker upload limit, and enable optional IAP NumPy optimization' \
	  '  make glean-vars                  Discover the active project, VM zones, and existing model GPU settings' \
	  '  make set-region OLD_ZONE=... TARGET_ZONE=... [SCOPE=app-db|all] [IMAGE_TAG=tag] [PUBLIC_HOST=host]  Migrate and deploy, then remove old registry copies' \
	  '  make artifact-registry-cleanup [KEEP_LOCATION=...]  Keep the region derived from GCP_ZONE by default' \
	  '  make infra-plan [GPU=on|off|auto] [GPU_TYPE=nvidia-tesla-t4] [MODEL_ONLY=1] [TF_PARALLELISM=n] Initialize, format, validate, and plan; default parallelism is 2' \
	  '  make infra-apply [GPU=on|off|auto] [GPU_TYPE=nvidia-tesla-t4] [MODEL_ONLY=1] [TF_AUTO_APPROVE=1] Initialize, format, validate, and apply; GPU affects only model VM' \
	  '  make tf-backend-setup           Create the shared GCS Terraform backend, collect admin emails until stop, and migrate state' \
	  '  make tf-backend-join             Connect this shell to the shared Terraform state' \
	  '  make ssh-machine MACHINE=app|database|model  SSH to a VM over IAP (interactive)' \
	  '  make grafana                         Open deployed Grafana through IAP (requires applied infrastructure and a deployed Grafana workload)' \
	  '  make grafana-stop                    Stop this local user'\''s tracked Grafana tunnels' \
	  '  make pause-app                       Scale website and model workloads to zero; keep VMs, databases and monitoring running' \
	  '  make resume-app                      Restore website and model workloads to one replica' \
	  '  make reboot MACHINE=app|database|model|all   Reboot one VM or all sequentially over IAP' \
	  '  make clear-databases CONFIRM_CLEAR_DATABASES=1  Erase and recreate the three PostgreSQL databases' \
	  '  make clear-databases-reboot-all CONFIRM_CLEAR_DATABASES=1  Clear PostgreSQL data, then reboot every VM' \
	  '  make cluster-bootstrap             Join worker VMs; automatically set up GPU runtime/plugin when GPU is enabled' \
	  '  make secrets-init                 Discover GCP settings and prepare local secrets/keys; does not provision infrastructure' \
	  '  make secrets-passwords            Fill placeholder DB passwords without rotating existing values' \
	  '  make docker-push-limit [DOCKER_MAX_CONCURRENT_UPLOADS=n] Limit layers uploaded per image push (requires sudo)' \
	  '  make docker-push-limit-reset      Remove the Docker upload limit and restore its default (requires sudo)' \
	  '  make build-push IMAGE_TAG=tag     Build/push images (does not start or stop VMs)' \
	  '  make k8s-config                   Ansible: create/update Kubernetes ConfigMaps and Secrets' \
	  '  make models                       Ansible: download and verify the model files on the model VM' \
	  '  make deploy IMAGE_TAG=tag [GPU=on|off|auto] Apply manifests and wait for rollouts' \
	  '  make update-app IMAGE_TAG=tag [GPU=on|off|auto] Build/push images, then deploy that tag' \
	  '  GPU_TYPE options: nvidia-tesla-t4 (default), nvidia-tesla-t4-vws, nvidia-l4, nvidia-l4-vws, nvidia-rtx-pro-6000, nvidia-rtx-pro-6000-vws' \
	  '                   nvidia-h100-80gb, nvidia-h100-mega-80gb, nvidia-h200-141gb, nvidia-b200' \
	  '  GPU_MACHINE_TYPE overrides the selected GPU type default; for N1+T4, GPU_COUNT selects 1, 2, or 4 GPUs' \
	  '  T4 is the default trial; L4 is a newer alternative. T4 support ends August 1, 2027; H100 Mega, H200, and B200 defaults use 8 GPUs' \
	  '  To choose a GPU type, use GPU=on GPU_TYPE=... on infra-plan; GPU=auto keeps the saved choice' \
	  '  Deployment steps, GPU options, and target reference: deploy/README.md#gpu-options' \
	  '  Enable: infra-plan GPU=on GPU_TYPE=...; infra-apply; cluster-bootstrap; deploy GPU=auto' \
	  '  Disable: deploy GPU=off first; then infra-plan GPU=off and infra-apply GPU=auto' \
	  '  make gpu-capacity [GPU_REGION=europe|us]  Try model-only T4/L4 applies in documented zones until one succeeds' \
	  '  make container-logs [WORKLOAD=name] CONTAINER=name [NAMESPACE=movierec] [FOLLOW=1]  Infer workload from container when omitted' \
	  '' \
	  'Performance:' \
	  '  make benchmark                    Run ApacheBench (AB_URL, AB_REQUESTS, AB_CONCURRENCY)' \
	  '  make benchmark-stress             Run health, movies, large-page, and genres stress profiles' \
	  '  make start                        Start existing VMs directly via gcloud (no Terraform state needed)' \
	  '  make stop                         Stop VMs directly via gcloud; disks/IP/registry remain' \
	  '  make destroy CONFIRM_DESTROY=1 [TF_PARALLELISM=n] Destroy Terraform resources and attached disks' \
	  '' \
	  'Ansible targets run kubectl on the app VM over IAP SSH; no local kubeconfig or tunnel is needed.'

cloud-shell-setup:
	bash $(GCP_SCRIPTS)/setup-cloud-shell.sh
	$(MAKE) docker-push-limit

glean-vars:
	$(GCP_CONFIG) glean-vars

# Don't re-glean during plans/applies: migration and GPU capacity trials intentionally edit terraform.tfvars.
infra-plan:
	$(TF_PREPARE)
	$(GCP_CONFIG) set-gpu-vars $(GPU_SET_VARS_ARGS)
	terraform -chdir=$(TF_DIR) plan $(TF_PARALLELISM_FLAG) $(TF_PLAN_FLAGS)

infra-apply:
	$(TF_PREPARE)
	@if [ "$(MODEL_ONLY)" = 1 ]; then \
	  printf '%s\n' 'Model-only apply: Terraform targets the model VM, its network dependencies, and K3s firewall rules; app/database VMs are not targeted.'; \
	else \
	  printf '%s\n' 'Warning: this creates and starts cloud VMs plus disks, NAT, a public IP, and Artifact Registry resources; charges begin while VMs run and retained resources can still incur charges after make stop.'; \
	fi
	$(GCP_CONFIG) set-gpu-vars $(GPU_SET_VARS_ARGS)
	terraform -chdir=$(TF_DIR) apply $(TF_PARALLELISM_FLAG) $(TF_APPLY_FLAGS)

tf-backend-setup:
	bash $(GCP_SCRIPTS)/terraform-backend.sh setup

tf-backend-join:
	bash $(GCP_SCRIPTS)/terraform-backend.sh join

ssh-machine:
	@test -n "$(MACHINE)" || { printf '%s\n' 'Set MACHINE to app, database, or model.'; exit 2; }
	bash $(GCP_SCRIPTS)/ssh-machine.sh "$(MACHINE)"

reboot:
	@test -n "$(MACHINE)" || { printf '%s\n' 'Set MACHINE to app, database, model, or all.'; exit 2; }
	bash $(GCP_SCRIPTS)/reboot-machine.sh "$(MACHINE)"

clear-databases:
	@test "$(CONFIRM_CLEAR_DATABASES)" = 1 || { printf '%s\n' 'Refusing to erase PostgreSQL data. Use: make clear-databases CONFIRM_CLEAR_DATABASES=1'; exit 2; }
	bash $(GCP_SCRIPTS)/clear-databases.sh

clear-databases-reboot-all:
	@test "$(CONFIRM_CLEAR_DATABASES)" = 1 || { printf '%s\n' 'Refusing to erase PostgreSQL data. Use: make clear-databases-reboot-all CONFIRM_CLEAR_DATABASES=1'; exit 2; }
	$(MAKE) clear-databases CONFIRM_CLEAR_DATABASES=1 && $(MAKE) reboot MACHINE=all

cluster-bootstrap:
	$(SSH_ACCESS)
	$(ANSIBLE_PLAY) playbooks/cluster.yml
	bash $(GCP_SCRIPTS)/setup-gpu-if-enabled.sh "$(GPU)"

secrets-init: glean-vars
	bash $(GCP_SCRIPTS)/init-secrets.sh
	$(MAKE) secrets-passwords

set-region:
	@test -n "$(OLD_ZONE)" || { printf '%s\n' 'Set OLD_ZONE to the current app/database zone.'; exit 2; }
	@test -n "$(TARGET_ZONE)" || { printf '%s\n' 'Set TARGET_ZONE to the destination zone (TARGET_MODEL_ZONE is also accepted).'; exit 2; }
	@test "$(SCOPE)" = app-db -o "$(SCOPE)" = all || { printf '%s\n' 'Set SCOPE to app-db or all.'; exit 2; }
	python3 $(GCP_SCRIPTS)/migrate-region.py --old-zone "$(OLD_ZONE)" --target-zone "$(TARGET_ZONE)" --scope "$(SCOPE)" $(if $(strip $(IMAGE_TAG)),--image-tag "$(IMAGE_TAG)",) $(if $(strip $(PUBLIC_HOST)),--public-host "$(PUBLIC_HOST)",)

artifact-registry-cleanup:
	python3 $(GCP_SCRIPTS)/cleanup-artifact-registry.py $(if $(strip $(KEEP_LOCATION)),--keep-location "$(KEEP_LOCATION)",)

secrets-passwords:
	python3 scripts/generate-db-passwords.py

docker-push-limit:
	sudo bash $(GCP_SCRIPTS)/set-docker-push-limit.sh "$(DOCKER_MAX_CONCURRENT_UPLOADS)"

docker-push-limit-reset:
	sudo bash $(GCP_SCRIPTS)/reset-docker-push-limit.sh

build-push:
	bash $(GCP_SCRIPTS)/build-and-push.sh "$(IMAGE_TAG)"

k8s-config:
	$(SSH_ACCESS)
	$(ANSIBLE_PLAY) playbooks/config.yml -e public_host="$(PUBLIC_HOST)"

grafana:
	$(SSH_ACCESS)
	bash $(GCP_SCRIPTS)/grafana-tunnel.sh start

grafana-stop:
	bash $(GCP_SCRIPTS)/grafana-tunnel.sh stop

MOVIEREC_APP_DEPLOYMENTS := deployment/frontend deployment/gateway deployment/auth deployment/movies \
	deployment/recommendations deployment/llama-embedding deployment/llama-generation

pause-app:
	$(SSH_ACCESS)
	bash $(GCP_SCRIPTS)/ssh-machine.sh app --command \
	  "sudo k3s kubectl -n movierec scale $(MOVIEREC_APP_DEPLOYMENTS) --replicas=0"

resume-app:
	$(SSH_ACCESS)
	bash $(GCP_SCRIPTS)/ssh-machine.sh app --command \
	  "sudo k3s kubectl -n movierec scale $(MOVIEREC_APP_DEPLOYMENTS) --replicas=1"

models:
	$(SSH_ACCESS)
	$(ANSIBLE_PLAY) playbooks/models.yml

deploy:
	$(SSH_ACCESS)
	@test -n "$(IMAGE_TAG)" || { printf '%s\n' 'Set IMAGE_TAG, for example: make deploy IMAGE_TAG=20261012'; exit 2; }
	$(ANSIBLE_PLAY) playbooks/deploy.yml -e image_tag="$(IMAGE_TAG)" -e public_host="$(PUBLIC_HOST)" -e gpu_mode="$(GPU)"

update-app:
	@test -n "$(IMAGE_TAG)" || { printf '%s\n' 'Set IMAGE_TAG, for example: make update-app IMAGE_TAG=20261012'; exit 2; }
	$(MAKE) build-push IMAGE_TAG="$(IMAGE_TAG)"
	$(MAKE) deploy IMAGE_TAG="$(IMAGE_TAG)" PUBLIC_HOST="$(PUBLIC_HOST)" GPU="$(GPU)"

gpu-capacity:
	bash $(GCP_SCRIPTS)/try-gpu-zones.sh --region "$(GPU_REGION)"

container-logs:
	@test -n "$(WORKLOAD)$(CONTAINER)" || { printf '%s\n' 'Set CONTAINER to infer the workload, or set WORKLOAD.'; exit 2; }
	bash $(GCP_SCRIPTS)/container-logs.sh $(if $(strip $(WORKLOAD)),"$(WORKLOAD)",) \
	  $(if $(strip $(CONTAINER)),--container "$(CONTAINER)",) \
	  --namespace "$(NAMESPACE)" --tail "$(TAIL_LINES)" \
	  $(if $(filter 1 true yes,$(FOLLOW)),--follow,)

benchmark:
	bash scripts/apachebench.sh

benchmark-stress:
	bash scripts/apachebench-stress.sh

start:
	bash $(GCP_SCRIPTS)/manage-vms.sh start

stop:
	bash $(GCP_SCRIPTS)/manage-vms.sh stop

destroy:
	@test "$(CONFIRM_DESTROY)" = 1 || { printf '%s\n' 'Refusing to destroy resources. Use: make destroy CONFIRM_DESTROY=1'; exit 2; }
	terraform -chdir=$(TF_DIR) init
	terraform -chdir=$(TF_DIR) plan -destroy $(TF_PARALLELISM_FLAG)
	terraform -chdir=$(TF_DIR) destroy $(TF_PARALLELISM_FLAG)
