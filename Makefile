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
TF_PARALLELISM ?= 2
TF_PARALLELISM_FLAG = $(if $(strip $(TF_PARALLELISM)),-parallelism=$(TF_PARALLELISM),)
DOCKER_MAX_CONCURRENT_UPLOADS ?= 1
GPU ?= auto
GPU_TYPE ?= nvidia-tesla-t4
GPU_MACHINE_TYPE ?=
GPU_COUNT ?= 1
GPU_REGION ?= europe
MODEL_ONLY ?= 0
TF_AUTO_APPROVE ?= 0
GPU_SET_VARS_ARGS = --mode "$(GPU)" $(if $(filter on,$(GPU)),--type "$(GPU_TYPE)" --machine-type "$(GPU_MACHINE_TYPE)" --count "$(GPU_COUNT)")
SCOPE ?= app-db
TARGET_ZONE ?= $(TARGET_MODEL_ZONE)
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
export GCP_PROJECT_ID GCP_REGION GCP_ZONE GCP_ARTIFACT_REGISTRY_REGION MOVIEREC_RESOURCE_NAME
export TF_PARALLELISM

.PHONY: help cloud-shell-setup infra-config infra-init infra-fmt infra-validate infra-plan infra-apply \
        ssh-access ssh-machine grafana pause-app resume-app reboot clear-databases clear-databases-reboot-all cluster-bootstrap secrets-init secrets-passwords set-region artifact-registry-cleanup docker-push-limit docker-push-limit-reset build-push \
        k8s-config models deploy update-app gpu-runtime gpu-setup gpu-capacity gpu-capacity-europe container-logs start stop destroy benchmark benchmark-stress

help:
	@printf '%s\n' \
	  'Cloud workflow:' \
	  '  make cloud-shell-setup            Install tools, limit Docker uploads to 2, and enable optional IAP NumPy optimization' \
	  '  make infra-config                Set project_id from gcloud; use Terraform locations, falling back to GCP_REGION/GCP_ZONE' \
	  '  make set-region OLD_ZONE=... TARGET_ZONE=... [SCOPE=app-db|all] [IMAGE_TAG=tag] [PUBLIC_HOST=host]  Migrate and deploy, then remove old registry copies' \
	  '  make artifact-registry-cleanup [KEEP_LOCATION=...]  Keep the region derived from GCP_ZONE by default' \
	  '  make infra-init                   Initialize Terraform (local state in Cloud Shell)' \
	  '  make infra-plan [GPU=on|off|auto] [GPU_TYPE=nvidia-tesla-t4] [MODEL_ONLY=1] [TF_PARALLELISM=n] Plan resources; default parallelism is 2' \
	  '  make infra-apply [GPU=on|off|auto] [GPU_TYPE=nvidia-tesla-t4] [MODEL_ONLY=1] [TF_AUTO_APPROVE=1] Apply resources; GPU affects only model VM' \
	  '  make ssh-machine MACHINE=app|database|model  SSH to a VM over IAP (interactive)' \
	  '  make grafana                         Open Grafana through IAP with a per-admin Cloud Shell preview tunnel' \
	  '  make pause-app                       Scale website and model workloads to zero; keep VMs, databases and monitoring running' \
	  '  make resume-app                      Restore website and model workloads to one replica' \
	  '  make reboot MACHINE=app|database|model|all   Reboot one VM or all sequentially over IAP' \
	  '  make clear-databases CONFIRM_CLEAR_DATABASES=1  Erase and recreate the three PostgreSQL databases' \
	  '  make clear-databases-reboot-all CONFIRM_CLEAR_DATABASES=1  Clear PostgreSQL data, then reboot every VM' \
	  '  make cluster-bootstrap             Join worker VMs; automatically set up GPU runtime/plugin when GPU is enabled' \
	  '  make secrets-init                 Create server.env, resolve the app zone, and prepare secrets/keys' \
	  '  make secrets-passwords            Generate matching random passwords for the three databases' \
	  '  make docker-push-limit [DOCKER_MAX_CONCURRENT_UPLOADS=n] Limit layers uploaded per image push (requires sudo)' \
	  '  make docker-push-limit-reset      Remove the Docker upload limit and restore its default (requires sudo)' \
	  '  make build-push [IMAGE_TAG=tag]   Build/push images (does not start or stop VMs)' \
	  '  make k8s-config                   Ansible: create/update Kubernetes ConfigMaps and Secrets' \
	  '  make models                       Ansible: download and verify the model files on the model VM' \
	  '  make deploy IMAGE_TAG=tag [GPU=on|off|auto] Apply manifests and wait for rollouts' \
	  '  make update-app IMAGE_TAG=tag [GPU=on|off|auto] Build/push images, then deploy that tag' \
	  '  GPU_TYPE options: nvidia-tesla-t4 (default), nvidia-tesla-t4-vws, nvidia-l4, nvidia-l4-vws, nvidia-rtx-pro-6000, nvidia-rtx-pro-6000-vws' \
	  '                   nvidia-h100-80gb, nvidia-h100-mega-80gb, nvidia-h200-141gb, nvidia-b200' \
	  '  GPU_MACHINE_TYPE overrides the selected GPU type default; for N1+T4, GPU_COUNT selects 1, 2, or 4 GPUs' \
	  '  T4 is the default trial; L4 is a newer alternative. T4 support ends August 1, 2027; H100 Mega, H200, and B200 defaults use 8 GPUs' \
	  '  To choose a GPU type, use GPU=on GPU_TYPE=... on infra-plan; GPU=auto keeps the saved choice' \
	  '  GPU steps, examples, and provisioning caveats: readmes/step_by_step_deploy.md#optional-enable-gpu-generation' \
	  '  Enable: infra-plan GPU=on GPU_TYPE=...; infra-apply; cluster-bootstrap; deploy GPU=auto' \
	  '  Disable: deploy GPU=off first; then infra-plan GPU=off and infra-apply GPU=auto' \
	  '  make gpu-runtime                  Install NVIDIA drivers/runtime on the model VM after infra-apply GPU=on' \
	  '  make gpu-setup                    Install the Kubernetes GPU device plugin after gpu-runtime' \
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
	$(MAKE) docker-push-limit DOCKER_MAX_CONCURRENT_UPLOADS=2

infra-config:
	bash $(GCP_SCRIPTS)/configure-terraform.sh

infra-init:
	terraform -chdir=$(TF_DIR) init

infra-fmt:
	terraform -chdir=$(TF_DIR) fmt

infra-validate: infra-init infra-fmt
	terraform -chdir=$(TF_DIR) validate

infra-plan: infra-validate
	$(GCP_CONFIG) set-gpu-vars $(GPU_SET_VARS_ARGS)
	terraform -chdir=$(TF_DIR) plan $(TF_PARALLELISM_FLAG) $(TF_PLAN_FLAGS)

infra-apply: infra-validate
	@if [ "$(MODEL_ONLY)" = 1 ]; then \
	  printf '%s\n' 'Model-only apply: Terraform targets the model VM, its network dependencies, and K3s firewall rules; app/database VMs are not targeted.'; \
	else \
	  printf '%s\n' 'Warning: this creates and starts cloud VMs plus disks, NAT, a public IP, and Artifact Registry resources; charges begin while VMs run and retained resources can still incur charges after make stop.'; \
	fi
	$(GCP_CONFIG) set-gpu-vars $(GPU_SET_VARS_ARGS)
	terraform -chdir=$(TF_DIR) apply $(TF_PARALLELISM_FLAG) $(TF_APPLY_FLAGS)

ssh-access:
	@PROJECT_ID="$$(bash $(GCP_SCRIPTS)/gcp-context.sh project-id)"; \
	APP_INSTANCE="$(MOVIEREC_RESOURCE_NAME)-app"; \
	ZONE="$(GCP_ZONE)"; \
	if [[ -z "$$PROJECT_ID" || "$$PROJECT_ID" == '(unset)' || -z "$$APP_INSTANCE" || -z "$$ZONE" ]]; then \
	  printf '%s\n' 'Set the GCP project, zone, and MovieRec resource name before connecting.' >&2; exit 2; \
	fi; \
	gcloud compute ssh "$$APP_INSTANCE" --project="$$PROJECT_ID" --zone="$$ZONE" --tunnel-through-iap --quiet --command=true

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

cluster-bootstrap: ssh-access
	$(ANSIBLE_PLAY) playbooks/cluster.yml
	bash $(GCP_SCRIPTS)/setup-gpu-if-enabled.sh "$(GPU)"

secrets-init:
	bash $(GCP_SCRIPTS)/init-secrets.sh

set-region:
	@test -n "$(OLD_ZONE)" || { printf '%s\n' 'Set OLD_ZONE to the current app/database zone.'; exit 2; }
	@test -n "$(TARGET_ZONE)" || { printf '%s\n' 'Set TARGET_ZONE to the destination zone (TARGET_MODEL_ZONE is also accepted).'; exit 2; }
	@test "$(SCOPE)" = app-db -o "$(SCOPE)" = all || { printf '%s\n' 'Set SCOPE to app-db or all.'; exit 2; }
	python3 $(GCP_SCRIPTS)/migrate-region.py --old-zone "$(OLD_ZONE)" --target-zone "$(TARGET_ZONE)" --scope "$(SCOPE)" $(if $(strip $(IMAGE_TAG)),--image-tag "$(IMAGE_TAG)",) $(if $(strip $(PUBLIC_HOST)),--public-host "$(PUBLIC_HOST)",)

artifact-registry-cleanup:
	python3 $(GCP_SCRIPTS)/cleanup-artifact-registry.py $(if $(strip $(KEEP_LOCATION)),--keep-location "$(KEEP_LOCATION)",)

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

grafana: ssh-access
	@grafana_local_port=$$((20000 + RANDOM % 20000)); \
	grafana_remote_port=$$((40000 + RANDOM % 20000)); \
	if [[ -n "$${WEB_HOST:-}" ]]; then \
	  [[ "$${WEB_HOST}" =~ ^[A-Za-z0-9.-]+$$ ]] || { printf '%s\n' 'WEB_HOST contains invalid characters.' >&2; exit 2; }; \
	  printf 'Grafana: https://%s-%s/\n' "$${grafana_local_port}" "$${WEB_HOST}"; \
	else \
	  printf 'Open http://localhost:%s in a local browser.\n' "$${grafana_local_port}"; \
	fi; \
	printf '%s\n' 'Configuring stable Grafana proxy settings; this only rolls out Grafana if they changed.' 'Press Ctrl-C to close the tunnel.'; \
	gcloud compute ssh "$(MOVIEREC_RESOURCE_NAME)-app" \
	  --project="$(GCP_PROJECT_ID)" --zone="$(GCP_ZONE)" --tunnel-through-iap --quiet \
	  --ssh-flag="-L 127.0.0.1:$${grafana_local_port}:localhost:$${grafana_remote_port}" \
	  --command="sudo k3s kubectl -n movierec set env deployment/grafana GF_SERVER_ROOT_URL=http://localhost:3000/ GF_SECURITY_CSRF_ADDITIONAL_HEADERS=X-Forwarded-Host 'GF_LIVE_ALLOWED_ORIGINS=https://*-cs-*.cloudshell.dev,http://localhost:*,http://127.0.0.1:*' GF_SECURITY_CSRF_TRUSTED_ORIGINS- && sudo k3s kubectl -n movierec rollout status deployment/grafana --timeout=5m && sudo k3s kubectl -n movierec port-forward service/grafana $${grafana_remote_port}:3000 --address 127.0.0.1"

MOVIEREC_APP_DEPLOYMENTS := deployment/frontend deployment/gateway deployment/auth deployment/movies \
	deployment/recommendations deployment/llama-embedding deployment/llama-generation

pause-app: ssh-access
	bash $(GCP_SCRIPTS)/ssh-machine.sh app --command \
	  "sudo k3s kubectl -n movierec scale $(MOVIEREC_APP_DEPLOYMENTS) --replicas=0"

resume-app: ssh-access
	bash $(GCP_SCRIPTS)/ssh-machine.sh app --command \
	  "sudo k3s kubectl -n movierec scale $(MOVIEREC_APP_DEPLOYMENTS) --replicas=1"

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

gpu-capacity:
	bash $(GCP_SCRIPTS)/try-gpu-zones.sh --region "$(GPU_REGION)"

gpu-capacity-europe:
	$(MAKE) gpu-capacity GPU_REGION=europe

container-logs:
	@test -n "$(WORKLOAD)$(CONTAINER)" || { printf '%s\n' 'Set CONTAINER to infer the workload, or set WORKLOAD.'; exit 2; }
	bash $(GCP_SCRIPTS)/container-logs.sh $(if $(strip $(WORKLOAD)),"$(WORKLOAD)",) \
	  $(if $(strip $(CONTAINER)),--container "$(CONTAINER)",) \
	  $(if $(strip $(NAMESPACE)),--namespace "$(NAMESPACE)",) \
	  $(if $(strip $(TAIL_LINES)),--tail "$(TAIL_LINES)",) \
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
	terraform -chdir=$(TF_DIR) plan -destroy $(TF_PARALLELISM_FLAG)
	terraform -chdir=$(TF_DIR) destroy $(TF_PARALLELISM_FLAG)
