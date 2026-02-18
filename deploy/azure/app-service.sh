#!/usr/bin/env bash
# Deploy Hangar Genesis to Azure App Service (Web App for Containers).
# Creates resource group, ACR, App Service plan, and Web App if they don't exist.
# Requires: Docker, Azure CLI.
# Usage:
#   1. Copy .env.example to .env and set at least AZURE_SUBSCRIPTION_ID, ACR_NAME, WEBAPP_NAME.
#   2. az login && az account set --subscription "<id>"
#   3. ./app-service.sh [create|deploy|all]

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
APP_SERVICE_PLAN_NAME="${APP_SERVICE_PLAN_NAME:-genesis-plan}"
WEBAPP_NAME="${WEBAPP_NAME:-}"

# Web App name must be globally unique (e.g. pse-genesis-xyz or your-company-genesis)
if [[ -z "$WEBAPP_NAME" ]]; then
  echo "Set WEBAPP_NAME in .env (e.g. pse-genesis-unique123)"
  exit 1
fi
if [[ -z "$ACR_NAME" ]]; then
  echo "Set ACR_NAME in .env for App Service (must push image to ACR)"
  exit 1
fi
if [[ -z "$AZURE_SUBSCRIPTION_ID" ]]; then
  echo "Set AZURE_SUBSCRIPTION_ID in .env"
  exit 1
fi

az account set --subscription "$AZURE_SUBSCRIPTION_ID"
FULL_IMAGE="${ACR_NAME}.azurecr.io/${IMAGE_NAME}:${IMAGE_TAG}"

do_build_push() {
  echo "Building Genesis image..."
  docker build -f deploy/azure/Dockerfile.genesis -t "${IMAGE_NAME}:${IMAGE_TAG}" .
  az acr login -n "$ACR_NAME"
  docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "$FULL_IMAGE"
  docker push "$FULL_IMAGE"
  echo "Pushed $FULL_IMAGE"
}

do_create() {
  echo "Creating resource group (if not exists)..."
  az group create --name "$RESOURCE_GROUP_NAME" --location "$AZURE_LOCATION" --output none 2>/dev/null || true

  echo "Creating ACR (if not exists)..."
  az acr show --name "$ACR_NAME" --output none 2>/dev/null || \
    az acr create --resource-group "$RESOURCE_GROUP_NAME" --name "$ACR_NAME" --sku Basic --output none

  echo "Creating App Service plan (if not exists)..."
  az appservice plan show --name "$APP_SERVICE_PLAN_NAME" --resource-group "$RESOURCE_GROUP_NAME" --output none 2>/dev/null || \
    az appservice plan create --resource-group "$RESOURCE_GROUP_NAME" --name "$APP_SERVICE_PLAN_NAME" \
      --is-linux --sku B1 --location "$AZURE_LOCATION" --output none

  echo "Creating Web App (if not exists)..."
  if az webapp show --name "$WEBAPP_NAME" --resource-group "$RESOURCE_GROUP_NAME" --output none 2>/dev/null; then
    echo "Web App $WEBAPP_NAME already exists."
  else
    az webapp create --resource-group "$RESOURCE_GROUP_NAME" --plan "$APP_SERVICE_PLAN_NAME" \
      --name "$WEBAPP_NAME" --deployment-container-image-name "$FULL_IMAGE" --output none
    # Enable ACR pull: use admin credentials (simplest); for production consider managed identity
    ACR_ID=$(az acr show --name "$ACR_NAME" --query id -o tsv)
    ACR_USER=$(az acr credential show --name "$ACR_NAME" --query username -o tsv)
    ACR_PASS=$(az acr credential show --name "$ACR_NAME" --query "passwords[0].value" -o tsv)
    az webapp config container set --resource-group "$RESOURCE_GROUP_NAME" --name "$WEBAPP_NAME" \
      --docker-custom-image-name "$FULL_IMAGE" \
      --docker-registry-server-url "https://${ACR_NAME}.azurecr.io" \
      --docker-registry-server-user "$ACR_USER" \
      --docker-registry-server-password "$ACR_PASS" \
      --output none
  fi

  # Ensure container listens on PORT (App Service sets PORT env, our binary uses 8080 by default)
  # Our Dockerfile uses --port=8080; App Service injects WEBSITES_PORT=8080 when the container exposes 8080
  az webapp config appsettings set --resource-group "$RESOURCE_GROUP_NAME" --name "$WEBAPP_NAME" \
    --settings WEBSITES_PORT=8080 --output none 2>/dev/null || true

  echo "App Service URL: https://${WEBAPP_NAME}.azurewebsites.net"
}

do_deploy() {
  do_build_push
  echo "Updating Web App to latest image..."
  az webapp config container set --resource-group "$RESOURCE_GROUP_NAME" --name "$WEBAPP_NAME" \
    --docker-custom-image-name "$FULL_IMAGE" \
    --output none
  az webapp restart --resource-group "$RESOURCE_GROUP_NAME" --name "$WEBAPP_NAME" --output none
  echo "Deployed. Open: https://${WEBAPP_NAME}.azurewebsites.net"
}

case "${1:-all}" in
  create) do_create ;;
  deploy) do_deploy ;;
  all)    do_build_push; do_create; do_deploy ;;
  *)      echo "Usage: $0 [create|deploy|all]"; exit 1 ;;
esac
