SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c

TF_DIR := deploy/gcp/terraform
GCP_SCRIPTS := scripts/gcp
GCP_CONFIG := python3 $(GCP_SCRIPTS)/gcp-config.py
# Resolve server.env and terraform.tfvars once; exported settings reach every recipe's child process.
GCP_CONFIG_ENV := $(shell $(GCP_CONFIG) make-vars)
ifeq ($(strip $(GCP_CONFIG_ENV)),)
$(error Failed to load GCP settings with $(GCP_CONFIG))
endif
$(foreach setting,$(GCP_CONFIG_ENV),$(eval $(subst =,:=,$(setting))))
TF_PARALLELISM ?=
TF_PARALLELISM_FLAG = $(if $(strip $(TF_PARALLELISM)),-parallelism=$(TF_PARALLELISM),)
DOCKER_MAX_CONCURRENT_UPLOADS ?= 1
GPU ?= auto
GPU_TYPE ?= nvidia-tesla-t4
GPU_MACHINE_TYPE ?=
GPU_COUNT ?= 1
GPU_CONFIG_MODE := $(shell $(GCP_CONFIG) gpu-mode --mode "$(GPU)" --type "$(GPU_TYPE)" --machine-type "$(GPU_MACHINE_TYPE)" --count "$(GPU_COUNT)")
ifneq ($(GPU_CONFIG_MODE),$(GPU))
$(error Invalid GPU settings; check GPU, GPU_TYPE, GPU_MACHINE_TYPE, and GPU_COUNT)
endif
TF_VAR_region := $(GCP_REGION)
TF_VAR_zone := $(GCP_ZONE)
TF_VAR_name ?= $(MOVIEREC_RESOURCE_NAME)
IMAGE_TAG ?=
PUBLIC_HOST ?=
GPU_LLAMA_IMAGE ?= ghcr.io/ggml-org/llama.cpp:server-cuda-v0.4.1
ANSIBLE_DIR := deploy/ansible
ANSIBLE_CONFIG_FILE := $(CURDIR)/$(ANSIBLE_DIR)/ansible.cfg
ANSIBLE_PLAY := cd $(ANSIBLE_DIR) && ANSIBLE_CONFIG="$(ANSIBLE_CONFIG_FILE)" ansible-playbook -i inventory.py

ifneq ($(strip $(TF_VAR_project_id)),)
export TF_VAR_project_id
endif
export TF_VAR_region TF_VAR_zone TF_VAR_name
export GCP_PROJECT_ID GCP_REGION GCP_ZONE MOVIEREC_RESOURCE_NAME

.PHONY: help cloud-shell-setup infra-config infra-init infra-fmt infra-validate infra-plan infra-apply \
        ssh-access cluster-bootstrap secrets-init secrets-passwords docker-push-limit docker-push-limit-reset build-push \
        k8s-config models deploy update-app gpu-runtime gpu-setup start stop destroy benchmark benchmark-stress

help:
	@printf '%s\n' \
	  'Cloud workflow:' \
	  '  make cloud-shell-setup            Install Cloud Shell tools and optional IAP NumPy optimization' \
	  '  make infra-config                Set project_id from gcloud; use Terraform locations, falling back to GCP_REGION/GCP_ZONE' \
	  '  make infra-init                   Initialize Terraform (local state in Cloud Shell)' \
	  '  make infra-plan [GPU=on|off|auto] [GPU_TYPE=nvidia-tesla-t4] [TF_PARALLELISM=n] Plan resources; GPU affects only model VM' \
	  '  make infra-apply [GPU=on|off|auto] [GPU_TYPE=nvidia-tesla-t4] [TF_PARALLELISM=n] Apply resources' \
	  '  make cluster-bootstrap             Ansible: join running database/model VMs to the app K3s server' \
	  '  make secrets-init                 Create server.env, fill its project ID, and prepare secrets/keys' \
	  '  make secrets-passwords            Generate matching random passwords for the three databases' \
	  '  make docker-push-limit [DOCKER_MAX_CONCURRENT_UPLOADS=n] Limit layers uploaded per image push (requires sudo)' \
	  '  make docker-push-limit-reset      Remove the Docker upload limit and restore its default (requires sudo)' \
	  '  make build-push [IMAGE_TAG=tag]   Build/push images (does not start or stop VMs)' \
	  '  make k8s-config                   Ansible: create/update Kubernetes ConfigMaps and Secrets' \
	  '  make models                       Ansible: download and verify the model files on the model VM' \
	  '  make deploy IMAGE_TAG=tag [GPU=on|off|auto] Apply manifests and wait for rollouts' \
	  '  make update-app IMAGE_TAG=tag [GPU=on|off|auto] Build/push images, then deploy that tag' \
	  '  GPU_TYPE options: nvidia-tesla-t4 (default), nvidia-l4, nvidia-rtx-pro-6000, nvidia-h100-80gb, nvidia-h100-mega-80gb, nvidia-h200-141gb, nvidia-b200; append -vws for supported workstation variants' \
	  '  GPU_MACHINE_TYPE overrides the selected GPU type default; for N1+T4, GPU_COUNT selects 1-4 GPUs' \
	  '  T4 and L4 are lower-cost options; T4 support ends August 1, 2027. H100 Mega, H200, and B200 defaults use 8-GPU machines' \
	  '  Explicit GPU=on/off on infra-plan or infra-apply is saved in terraform.tfvars; GPU=auto keeps that saved choice' \
	  '  Enable: infra-plan GPU=on GPU_TYPE=...; infra-apply; gpu-runtime; gpu-setup; deploy GPU=auto' \
	  '  Disable: deploy GPU=off first; then infra-plan GPU=off and infra-apply GPU=auto' \
	  '  make gpu-runtime                  Install NVIDIA drivers/runtime on the model VM after infra-apply GPU=on' \
	  '  make gpu-setup                    Install the Kubernetes GPU device plugin after gpu-runtime' \
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

infra-config:
	bash $(GCP_SCRIPTS)/configure-terraform.sh

infra-init:
	terraform -chdir=$(TF_DIR) init

infra-fmt:
	terraform -chdir=$(TF_DIR) fmt

infra-validate: infra-init infra-fmt
	terraform -chdir=$(TF_DIR) validate

infra-plan: infra-validate
	$(GCP_CONFIG) set-gpu-vars --mode "$(GPU)" --type "$(GPU_TYPE)" --machine-type "$(GPU_MACHINE_TYPE)" --count "$(GPU_COUNT)"
	terraform -chdir=$(TF_DIR) plan $(TF_PARALLELISM_FLAG)

infra-apply: infra-validate
	@printf '%s\n' 'Warning: this creates and starts three cloud VMs plus disks, NAT, a public IP, and Artifact Registry resources; charges begin while VMs run and retained resources can still incur charges after make stop.'
	$(GCP_CONFIG) set-gpu-vars --mode "$(GPU)" --type "$(GPU_TYPE)" --machine-type "$(GPU_MACHINE_TYPE)" --count "$(GPU_COUNT)"
	terraform -chdir=$(TF_DIR) apply $(TF_PARALLELISM_FLAG)

ssh-access:
	@PROJECT_ID="$$(bash $(GCP_SCRIPTS)/gcp-context.sh project-id)"; \
	APP_INSTANCE="$(MOVIEREC_RESOURCE_NAME)-app"; \
	ZONE="$(GCP_ZONE)"; \
	if [[ -z "$$PROJECT_ID" || "$$PROJECT_ID" == '(unset)' || -z "$$APP_INSTANCE" || -z "$$ZONE" ]]; then \
	  printf '%s\n' 'Set the GCP project, zone, and MovieRec resource name before connecting.' >&2; exit 2; \
	fi; \
	gcloud compute ssh "$$APP_INSTANCE" --project="$$PROJECT_ID" --zone="$$ZONE" --tunnel-through-iap --quiet --command=true

cluster-bootstrap: ssh-access
	$(ANSIBLE_PLAY) playbooks/cluster.yml

secrets-init:
	bash $(GCP_SCRIPTS)/init-secrets.sh

secrets-passwords:
	python3 $(GCP_SCRIPTS)/generate-db-passwords.py

docker-push-limit:
	sudo bash $(GCP_SCRIPTS)/set-docker-push-limit.sh "$(DOCKER_MAX_CONCURRENT_UPLOADS)"

docker-push-limit-reset:
	sudo bash $(GCP_SCRIPTS)/reset-docker-push-limit.sh

build-push:
	bash $(GCP_SCRIPTS)/build-and-push.sh "$(IMAGE_TAG)"

k8s-config: ssh-access
	$(ANSIBLE_PLAY) playbooks/config.yml -e public_host="$(PUBLIC_HOST)"

models: ssh-access
	$(ANSIBLE_PLAY) playbooks/models.yml

deploy: ssh-access
	@test -n "$(IMAGE_TAG)" || { printf '%s\n' 'Set IMAGE_TAG, for example: make deploy IMAGE_TAG=20261012'; exit 2; }
	$(ANSIBLE_PLAY) playbooks/deploy.yml -e image_tag="$(IMAGE_TAG)" -e public_host="$(PUBLIC_HOST)" -e gpu_mode="$(GPU)"

update-app:
	@test -n "$(IMAGE_TAG)" || { printf '%s\n' 'Set IMAGE_TAG, for example: make update-app IMAGE_TAG=20261012'; exit 2; }
	$(MAKE) build-push IMAGE_TAG="$(IMAGE_TAG)"
	$(MAKE) deploy IMAGE_TAG="$(IMAGE_TAG)" PUBLIC_HOST="$(PUBLIC_HOST)" GPU="$(GPU)"

gpu-runtime:
	bash $(GCP_SCRIPTS)/install-gpu-runtime.sh

gpu-setup: ssh-access
	$(ANSIBLE_PLAY) playbooks/gpu-setup.yml

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
	terraform -chdir=$(TF_DIR) plan -destroy $(TF_PARALLELISM_FLAG)
	terraform -chdir=$(TF_DIR) destroy $(TF_PARALLELISM_FLAG)
