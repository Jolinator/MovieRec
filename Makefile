SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c

ROOT_DIR := $(abspath .)
TF_DIR := deploy/gcp/terraform
GCP_SCRIPTS := scripts/gcp
KUBECONFIG ?= $(ROOT_DIR)/kubeconfig
IMAGE_TAG ?=
PUBLIC_HOST ?=

export KUBECONFIG

.PHONY: help cloud-shell-setup infra-config infra-init infra-fmt infra-validate infra-plan infra-apply \
        secrets-init build-push connect k8s-config models deploy start stop destroy

help:
	@printf '%s\n' \
	  'Cloud workflow:' \
	  '  make cloud-shell-setup            Install Cloud Shell tools' \
	  '  make infra-config                Set project_id from gcloud; run before plan/apply' \
	  '  make infra-init                   Initialize Terraform' \
	  '  make infra-plan                   Validate and review the infrastructure plan' \
	  '  make infra-apply                  Create/start the cloud resources' \
	  '  make secrets-init                 Prepare ignored cloud secret files and cryptographic keys' \
	  '  make build-push [IMAGE_TAG=tag]   Build and push backend/frontend images' \
	  '  make connect                      Download kubeconfig and open the IAP tunnel' \
	  '  make k8s-config                   Create/update Kubernetes ConfigMaps and Secrets (after secrets-init)' \
	  '  make models                       Download and verify the model files' \
	  '  make deploy IMAGE_TAG=tag         Apply manifests and wait for rollouts' \
	  '  make start                        Start the existing VM' \
	  '  make stop                         Stop the existing VM' \
	  '  make destroy CONFIRM_DESTROY=1    Destroy Terraform resources' \
	  '' \
	  'Keep the IAP tunnel from make connect open while running k8s-config, models, or deploy.'

cloud-shell-setup:
	bash $(GCP_SCRIPTS)/setup-cloud-shell.sh

infra-config:
	bash $(GCP_SCRIPTS)/configure-terraform.sh

infra-init:
	terraform -chdir=$(TF_DIR) init

infra-fmt:
	terraform -chdir=$(TF_DIR) fmt -check

infra-validate: infra-init infra-fmt
	terraform -chdir=$(TF_DIR) validate

infra-plan: infra-validate
	terraform -chdir=$(TF_DIR) plan

infra-apply: infra-validate
	@printf '%s\n' 'Warning: this creates and starts the cloud VM(s), which can incur charges.'
	terraform -chdir=$(TF_DIR) apply

secrets-init:
	bash $(GCP_SCRIPTS)/init-secrets.sh

build-push:
	bash $(GCP_SCRIPTS)/build-and-push.sh "$(IMAGE_TAG)"

connect:
	bash $(GCP_SCRIPTS)/connect-k3s.sh

k8s-config:
	PUBLIC_HOST="$(PUBLIC_HOST)" bash $(GCP_SCRIPTS)/configure-k8s.sh

models:
	kubectl apply -f deploy/k8s/models.yaml
	kubectl -n movierec wait --for=condition=complete job/model-downloader --timeout=35m

deploy:
	@test -n "$(IMAGE_TAG)" || { printf '%s\n' 'Set IMAGE_TAG, for example: make deploy IMAGE_TAG=20261012'; exit 2; }
	PUBLIC_HOST="$(PUBLIC_HOST)" bash $(GCP_SCRIPTS)/apply-k8s.sh "$(IMAGE_TAG)"

start:
	INSTANCE="$$(terraform -chdir=$(TF_DIR) output -raw instance_name)"; \
	ZONE="$$(terraform -chdir=$(TF_DIR) output -raw zone)"; \
	gcloud compute instances start "$$INSTANCE" --zone="$$ZONE"; \
	gcloud compute instances describe "$$INSTANCE" --zone="$$ZONE" --format='value(status)'

stop:
	INSTANCE="$$(terraform -chdir=$(TF_DIR) output -raw instance_name)"; \
	ZONE="$$(terraform -chdir=$(TF_DIR) output -raw zone)"; \
	gcloud compute instances stop "$$INSTANCE" --zone="$$ZONE"

destroy:
	@test "$(CONFIRM_DESTROY)" = 1 || { printf '%s\n' 'Refusing to destroy resources. Use: make destroy CONFIRM_DESTROY=1'; exit 2; }
	terraform -chdir=$(TF_DIR) plan -destroy
	terraform -chdir=$(TF_DIR) destroy
