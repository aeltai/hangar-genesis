#!/usr/bin/env bash
# Deploy Hangar Genesis (backend + frontend) to Azure AKS.
# Requires: Docker, Azure CLI, kubectl.
# Usage:
#   1. Copy .env.example to .env and set variables (do not commit .env).
#   2. Log in: az login
#   3. Run: ./deploy.sh [build|push|apply|all]

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

if [[ -f "$SCRIPT_DIR/.env" ]]; then
  set -a
  source "$SCRIPT_DIR/.env"
  set +a
fi

# Defaults
AZURE_LOCATION="${AZURE_LOCATION:-westeurope}"
RESOURCE_GROUP_NAME="${RESOURCE_GROUP_NAME:-}"
AKS_CLUSTER_NAME="${AKS_CLUSTER_NAME:-}"
IMAGE_NAME="${IMAGE_NAME:-hangar-genesis}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
ACR_NAME="${ACR_NAME:-}"

do_build() {
  echo "Building Genesis image..."
  docker build -f --platform linux/amd64 deploy/azure/Dockerfile.genesis -t "${IMAGE_NAME}:${IMAGE_TAG}" .
}

do_push() {
  if [[ -z "$ACR_NAME" ]]; then
    echo "ACR_NAME not set; skipping push. Image will be loaded into kind/minikube or use a different registry."
    return 0
  fi
  az acr login -n "$ACR_NAME"
  REMOTE="${ACR_NAME}.azurecr.io/${IMAGE_NAME}:${IMAGE_TAG}"
  docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "$REMOTE"
  docker push "$REMOTE"
  echo "Pushed $REMOTE"
}

do_apply() {
  if [[ -z "$AZURE_SUBSCRIPTION_ID" ]] || [[ -z "$RESOURCE_GROUP_NAME" ]] || [[ -z "$AKS_CLUSTER_NAME" ]]; then
    echo "Set AZURE_SUBSCRIPTION_ID, RESOURCE_GROUP_NAME, AKS_CLUSTER_NAME (e.g. in .env)"
    exit 1
  fi
  az account set --subscription "$AZURE_SUBSCRIPTION_ID"
  az aks get-credentials --resource-group "$RESOURCE_GROUP_NAME" --name "$AKS_CLUSTER_NAME" --overwrite-existing

  MANIFEST_DIR="$SCRIPT_DIR/k8s"
  kubectl apply -f "$MANIFEST_DIR/deployment.yaml"

  if [[ -n "$ACR_NAME" ]]; then
    kubectl set image deployment/hangar-genesis "genesis=${ACR_NAME}.azurecr.io/${IMAGE_NAME}:${IMAGE_TAG}" -n hangar-genesis
    kubectl rollout status deployment/hangar-genesis -n hangar-genesis --timeout=120s
  fi

  echo "Deployment applied. Check: kubectl get pods -n hangar-genesis"
  echo "If using ingress: set GENESIS_INGRESS_HOST in .env or edit k8s/ingress.yaml"
}

case "${1:-all}" in
  build) do_build ;;
  push)  do_build; do_push ;;
  apply) do_apply ;;
  all)   do_build; do_push; do_apply ;;
  *)     echo "Usage: $0 [build|push|apply|all]"; exit 1 ;;
esac
