SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c

TF_DIR := deploy/gcp/terraform
TF_PARALLELISM ?=
TF_PARALLELISM_FLAG = $(if $(strip $(TF_PARALLELISM)),-parallelism=$(TF_PARALLELISM),)
DOCKER_MAX_CONCURRENT_UPLOADS ?= 1
GCP_SCRIPTS := scripts/gcp
IMAGE_TAG ?=
PUBLIC_HOST ?=
GPU_LLAMA_IMAGE ?= ghcr.io/ggml-org/llama.cpp:server-cuda-v0.4.1
GPU_MACHINE_TYPE ?= g2-standard-4
ANSIBLE_DIR := deploy/ansible
ANSIBLE_PLAY := cd $(ANSIBLE_DIR) && ansible-playbook

.PHONY: help cloud-shell-setup infra-config infra-init infra-fmt infra-validate infra-plan infra-apply \
        ssh-access cluster-bootstrap secrets-init secrets-passwords docker-push-limit docker-push-limit-reset build-push \
        k8s-config models deploy update-app gpu-enable gpu-runtime gpu-setup deploy-gpu start stop destroy

help:
	@printf '%s\n' \
	  'Cloud workflow:' \
	  '  make cloud-shell-setup            Install Cloud Shell tools and optional IAP NumPy optimization' \
	  '  make infra-config                Set project_id from gcloud; run before plan/apply' \
	  '  make infra-init                   Initialize Terraform (local state in Cloud Shell)' \
	  '  make infra-plan [TF_PARALLELISM=n] Validate/review plan (default: Terraform, 10 operations)' \
	  '  make infra-apply [TF_PARALLELISM=n] Create/start resources (default: Terraform, 10 operations)' \
	  '  make cluster-bootstrap             Ansible: join running database/model VMs to the app K3s server' \
	  '  make secrets-init                 Prepare ignored cloud secret files and cryptographic keys' \
	  '  make secrets-passwords            Generate matching random passwords for the three databases' \
	  '  make docker-push-limit [DOCKER_MAX_CONCURRENT_UPLOADS=n] Limit layers uploaded per image push (requires sudo)' \
	  '  make docker-push-limit-reset      Remove the Docker upload limit and restore its default (requires sudo)' \
	  '  make build-push [IMAGE_TAG=tag]   Build/push images (does not start or stop VMs)' \
	  '  make k8s-config                   Ansible: create/update Kubernetes ConfigMaps and Secrets' \
	  '  make models                       Ansible: download and verify the model files on the model VM' \
	  '  make deploy IMAGE_TAG=tag         Ansible: apply the Kustomize manifests and wait for rollouts' \
	  '  make update-app IMAGE_TAG=tag     Build/push the app images, then deploy that tag' \
	  '  make gpu-enable [GPU_MACHINE_TYPE=g2-standard-4] Set GPU Terraform options (opt-in)' \
	  '  make gpu-runtime                  Install NVIDIA drivers/runtime on the model VM (opt-in)' \
	  '  make gpu-setup                    Ansible: install the Kubernetes GPU device plugin (opt-in)' \
	  '  make deploy-gpu IMAGE_TAG=tag     Ansible: apply the CUDA generation model overlay (opt-in)' \
	  '' \
	  'Performance:' \
	  '  make benchmark                    Run ApacheBench (AB_URL, AB_REQUESTS, AB_CONCURRENCY)' \
	  '  make benchmark-stress             Run health, movies, large-page, and genres stress profiles' \
	  '  make start                        Start all three existing VMs' \
	  '  make stop                         Stop VMs; disks/IP/registry remain' \
	  '  make destroy CONFIRM_DESTROY=1    Destroy Terraform resources and attached disks' \
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
	terraform -chdir=$(TF_DIR) plan $(TF_PARALLELISM_FLAG)

infra-apply: infra-validate
	@printf '%s\n' 'Warning: this creates and starts three cloud VMs plus disks, NAT, a public IP, and Artifact Registry resources; charges begin while VMs run and retained resources can still incur charges after make stop.'
	terraform -chdir=$(TF_DIR) apply $(TF_PARALLELISM_FLAG)

ssh-access:
	@APP_INSTANCE="$$(terraform -chdir=$(TF_DIR) output -raw app_instance_name 2>/dev/null)" || { \
	  printf '%s\n' 'Terraform has no app_instance_name output. Run make infra-apply first and confirm it completed successfully.' >&2; exit 2; \
	}; \
	ZONE="$$(terraform -chdir=$(TF_DIR) output -raw zone 2>/dev/null)" || { \
	  printf '%s\n' 'Terraform has no zone output. Run make infra-apply first and confirm it completed successfully.' >&2; exit 2; \
	}; \
	if [[ -z "$$APP_INSTANCE" || -z "$$ZONE" ]]; then \
	  printf '%s\n' 'Terraform returned an empty instance name or zone. Check the Terraform state and outputs.' >&2; exit 2; \
	fi; \
	gcloud compute ssh "$$APP_INSTANCE" --zone="$$ZONE" --tunnel-through-iap --quiet --command=true

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
	$(ANSIBLE_PLAY) playbooks/deploy.yml -e image_tag="$(IMAGE_TAG)" -e public_host="$(PUBLIC_HOST)"

update-app:
	@test -n "$(IMAGE_TAG)" || { printf '%s\n' 'Set IMAGE_TAG, for example: make update-app IMAGE_TAG=20261012'; exit 2; }
	$(MAKE) build-push IMAGE_TAG="$(IMAGE_TAG)"
	$(MAKE) deploy IMAGE_TAG="$(IMAGE_TAG)" PUBLIC_HOST="$(PUBLIC_HOST)"

gpu-enable:
	bash $(GCP_SCRIPTS)/enable-gpu.sh "$(GPU_MACHINE_TYPE)"

gpu-runtime:
	bash $(GCP_SCRIPTS)/install-gpu-runtime.sh

gpu-setup: ssh-access
	$(ANSIBLE_PLAY) playbooks/gpu-setup.yml

deploy-gpu: ssh-access
	@test -n "$(IMAGE_TAG)" || { printf '%s\n' 'Set IMAGE_TAG, for example: make deploy-gpu IMAGE_TAG=20261012'; exit 2; }
	$(ANSIBLE_PLAY) playbooks/deploy.yml -e image_tag="$(IMAGE_TAG)" -e public_host="$(PUBLIC_HOST)" -e gpu_overlay=true -e gpu_llama_image="$(GPU_LLAMA_IMAGE)"

benchmark:
	bash scripts/apachebench.sh

benchmark-stress:
	bash scripts/apachebench-stress.sh

start:
	ZONE="$$(terraform -chdir=$(TF_DIR) output -raw zone)"; \
	STATUS=0; \
	for INSTANCE in "$$(terraform -chdir=$(TF_DIR) output -raw app_instance_name)" "$$(terraform -chdir=$(TF_DIR) output -raw database_instance_name)" "$$(terraform -chdir=$(TF_DIR) output -raw model_instance_name)"; do \
	  if ! INSTANCE_STATUS="$$(gcloud compute instances describe "$$INSTANCE" --zone="$$ZONE" --format='value(status)' 2>/dev/null)"; then \
	    printf '%s\n' "Could not read status for $$INSTANCE" >&2; STATUS=1; continue; \
	  fi; \
	  if [[ "$$INSTANCE_STATUS" == RUNNING || "$$INSTANCE_STATUS" == PROVISIONING || "$$INSTANCE_STATUS" == STAGING ]]; then \
	    printf '%s\n' "$$INSTANCE already running (status: $$INSTANCE_STATUS)"; continue; \
	  fi; \
	  if ! gcloud compute instances start "$$INSTANCE" --zone="$$ZONE"; then \
	    printf '%s\n' "Failed to start $$INSTANCE" >&2; STATUS=1; continue; \
	  fi; \
	  if ! gcloud compute instances describe "$$INSTANCE" --zone="$$ZONE" --format='value(status)'; then \
	    printf '%s\n' "Could not read status for $$INSTANCE" >&2; STATUS=1; \
	  fi; \
	done; \
	exit "$$STATUS"

stop:
	ZONE="$$(terraform -chdir=$(TF_DIR) output -raw zone)"; \
	STATUS=0; \
	for INSTANCE in "$$(terraform -chdir=$(TF_DIR) output -raw app_instance_name)" "$$(terraform -chdir=$(TF_DIR) output -raw database_instance_name)" "$$(terraform -chdir=$(TF_DIR) output -raw model_instance_name)"; do \
	  if ! INSTANCE_STATUS="$$(gcloud compute instances describe "$$INSTANCE" --zone="$$ZONE" --format='value(status)' 2>/dev/null)"; then \
	    printf '%s\n' "Could not read status for $$INSTANCE" >&2; STATUS=1; continue; \
	  fi; \
	  if [[ "$$INSTANCE_STATUS" == TERMINATED ]]; then \
	    printf '%s\n' "$$INSTANCE is already stopped"; continue; \
	  fi; \
	  if ! gcloud compute instances stop "$$INSTANCE" --zone="$$ZONE"; then \
	    printf '%s\n' "Failed to stop $$INSTANCE; check its state because it may still be billing" >&2; STATUS=1; \
	  fi; \
	done; \
	exit "$$STATUS"

destroy:
	@test "$(CONFIRM_DESTROY)" = 1 || { printf '%s\n' 'Refusing to destroy resources. Use: make destroy CONFIRM_DESTROY=1'; exit 2; }
	terraform -chdir=$(TF_DIR) plan -destroy
	terraform -chdir=$(TF_DIR) destroy
