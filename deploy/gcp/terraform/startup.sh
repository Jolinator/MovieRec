#!/bin/bash
set -euxo pipefail
exec > >(tee -a /var/log/movierec-bootstrap.log) 2>&1

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y curl

PUBLIC_IP="$(curl -fsS -H 'Metadata-Flavor: Google' http://metadata.google.internal/computeMetadata/v1/instance/attributes/k3s_tls_san)"
curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=stable sh -s - server \
  --secrets-encryption \
  --tls-san "${PUBLIC_IP}" \
  --node-name movierec-node

until kubectl get nodes >/dev/null 2>&1; do sleep 5; done
kubectl wait --for=condition=Ready nodes --all --timeout=300s
kubectl get nodes
