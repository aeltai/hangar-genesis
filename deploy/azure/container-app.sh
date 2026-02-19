#!/usr/bin/env bash
# Deploy Hangar Genesis to Azure Container Apps using az containerapp up.
# Creates Container Apps environment and deploys the container from ACR.
# Requires: Docker, Azure CLI with containerapp extension.
# Usage:
#   1. cp .env.example .env and set AZURE_SUBSCRIPTION_ID, RESOURCE_GROUP_NAME, ACR_NAME.
#   2. az login && az account set --subscription "<id>"
#   3. bash container-app.sh [up|build-up|all]   (or: chmod +x container-app.sh && ./container-app.sh)

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

if [[ -f "$SCRIPT_DIR/.env" ]]; then
  set -a
  source "$SCRIPT_DIR/.env"
  set +a
fi

AZURE_LOCATION="${AZURE_LOCATION:-westeurope}"
RESOURCE_GROUP_NAME="${RESOURCE_GROUP_NAME:-pse-aeltai-aks-rg25}"
ACR_NAME="${ACR_NAME:-}"
IMAGE_NAME="${IMAGE_NAME:-hangar-genesis}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
CONTAINERAPP_NAME="${CONTAINERAPP_NAME:-genesis-app}"
CONTAINERAPP_ENV="${CONTAINERAPP_ENV:-genesis-env}"

if [[ -z "$ACR_NAME" ]]; then
  echo "Missing ACR_NAME. Copy .env.example to .env and set ACR_NAME (and RESOURCE_GROUP_NAME):"
  echo "  cp .env.example .env && <edit .env>"
  exit 1
fi

# Use current default subscription if unset or still the placeholder from docs
if [[ -z "$AZURE_SUBSCRIPTION_ID" ]] || [[ "$AZURE_SUBSCRIPTION_ID" == *"<"*">"* ]] || [[ "$AZURE_SUBSCRIPTION_ID" == "<id>" ]]; then
  AZURE_SUBSCRIPTION_ID=$(az account show -o tsv --query id 2>/dev/null || true)
  if [[ -z "$AZURE_SUBSCRIPTION_ID" ]]; then
    echo "Missing AZURE_SUBSCRIPTION_ID. Set it in .env to your subscription ID (not the literal '<id>')."
    echo "  Example: az account list -o table   # then put the subscription ID in .env"
    exit 1
  fi
  echo "Using current Azure subscription: $AZURE_SUBSCRIPTION_ID"
fi

az account set --subscription "$AZURE_SUBSCRIPTION_ID"
FULL_IMAGE="${ACR_NAME}.azurecr.io/${IMAGE_NAME}:${IMAGE_TAG}"

# Ensure Container Apps extension
az extension add --name containerapp --upgrade 2>/dev/null || true

do_build_push() {
  echo "Creating resource group and ACR if missing..."
  az group create --name "$RESOURCE_GROUP_NAME" --location "$AZURE_LOCATION" --output none 2>/dev/null || true
  az acr show --name "$ACR_NAME" --output none 2>/dev/null || \
    az acr create --resource-group "$RESOURCE_GROUP_NAME" --name "$ACR_NAME" --sku Basic --output none
  echo "Building Genesis image..."
  docker build --platform linux/amd64 -f deploy/azure/Dockerfile.genesis -t "${IMAGE_NAME}:${IMAGE_TAG}" .
  az acr login -n "$ACR_NAME"
  docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "$FULL_IMAGE"
  docker push "$FULL_IMAGE"
  echo "Pushed $FULL_IMAGE"
}

do_up() {
  echo "Deploying Container App (environment may be created if missing)..."

  # Register providers if not already (idempotent)
  echo "Ensuring Azure providers are registered..."
  az provider register --namespace Microsoft.App --wait 2>/dev/null || true
  az provider register --namespace Microsoft.OperationalInsights --wait 2>/dev/null || true

  echo "Ensuring ACR admin user is enabled (required for credential show)..."
  az acr update -n "$ACR_NAME" --admin-enabled true --output none

  echo "Getting ACR credentials..."
  ACR_USER=$(az acr credential show --name "$ACR_NAME" --query username -o tsv) || { echo "Error: failed to get ACR username. Check ACR_NAME and 'az acr credential show' permissions."; exit 1; }
  ACR_PASS=$(az acr credential show --name "$ACR_NAME" --query "passwords[0].value" -o tsv) || { echo "Error: failed to get ACR password."; exit 1; }

  echo "Running az containerapp up..."
  if ! az containerapp up \
    --name "$CONTAINERAPP_NAME" \
    --resource-group "$RESOURCE_GROUP_NAME" \
    --location "$AZURE_LOCATION" \
    --environment "$CONTAINERAPP_ENV" \
    --image "$FULL_IMAGE" \
    --target-port 8080 \
    --ingress external \
    --registry-server "${ACR_NAME}.azurecr.io" \
    --registry-username "$ACR_USER" \
    --registry-password "$ACR_PASS"; then
    echo "az containerapp up failed. If the app already exists, trying update..."
    az containerapp update --name "$CONTAINERAPP_NAME" --resource-group "$RESOURCE_GROUP_NAME" \
      --image "$FULL_IMAGE" --output none || true
  fi

  # Pass GitHub token from local .env into the container app (avoids API rate limits)
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    echo "Setting GITHUB_TOKEN on container app..."
    az containerapp update --name "$CONTAINERAPP_NAME" --resource-group "$RESOURCE_GROUP_NAME" \
      --set-env-vars "GITHUB_TOKEN=$GITHUB_TOKEN" --output none
  fi

  FQDN=$(az containerapp show --name "$CONTAINERAPP_NAME" --resource-group "$RESOURCE_GROUP_NAME" \
    --query "properties.configuration.ingress.fqdn" -o tsv 2>/dev/null || true)
  if [[ -n "$FQDN" ]]; then
    echo "Deployed. Open: https://${FQDN}"
  else
    echo "Container app name: $CONTAINERAPP_NAME. List apps: az containerapp list -g $RESOURCE_GROUP_NAME -o table"
  fi
}

case "${1:-all}" in
  up)        do_up ;;
  build-up)  do_build_push; do_up ;;
  all)       do_build_push; do_up ;;
  *)         echo "Usage: $0 [up|build-up|all]"; exit 1 ;;
esac
