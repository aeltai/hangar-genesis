# Deploy Hangar Genesis to Azure

This deploys **both** the Genesis API (Go backend) and the Vue.js frontend as a single container. You can use **Azure Container Apps** (serverless, recommended), **Azure App Service**, or **AKS** (Kubernetes).

## Security notice

**Do not commit real credentials.** The values you use for Rancher, Azure client secret, or tokens should be set only in a local `.env` file or environment. If you have already pasted secrets in chat or in a file, **rotate them** (revoke and create new tokens / client secrets).

## Prerequisites

- Docker
- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) (`az`)
- For AKS only: [kubectl](https://kubernetes.io/docs/tasks/tools/) and an existing AKS cluster
- For Container Apps: `az extension add --name containerapp --upgrade`

---

## Option 1: Azure Container Apps (recommended, serverless)

Uses `az containerapp up`: creates the Container Apps environment (and Log Analytics if needed) and deploys your image from ACR. No Kubernetes or App Service plan to manage.

1. **Copy env and set values:**

   ```bash
   cd deploy/azure
   cp .env.example .env
   # Edit .env: AZURE_SUBSCRIPTION_ID, RESOURCE_GROUP_NAME, ACR_NAME
   # Optional: CONTAINERAPP_NAME (default genesis-app), CONTAINERAPP_ENV (default genesis-env)
   ```

2. **Install extension and log in:**

   ```bash
   az extension add --name containerapp --upgrade
   az login
   az account set --subscription "<your-subscription-id>"
   ```

3. **Create ACR and push image, then deploy:**

   ```bash
   chmod +x container-app.sh
   ./container-app.sh all
   ```

4. **Open the app:** The script prints the FQDN (e.g. `https://genesis-app.<hash>.<region>.azurecontainerapps.io`).

**Commands:**

- `./container-app.sh all` – build image, push to ACR, run `az containerapp up` (create/update app and environment)
- `./container-app.sh build-up` – same as all
- `./container-app.sh up` – deploy existing image from ACR (no build)

---

## Option 2: Azure App Service

No Kubernetes required. The script creates a resource group, Azure Container Registry (ACR), App Service plan, and Web App, then deploys the Genesis container.

1. **Copy env and set values:**

   ```bash
   cd deploy/azure
   cp .env.example .env
   # Edit .env: set AZURE_SUBSCRIPTION_ID, RESOURCE_GROUP_NAME, ACR_NAME, WEBAPP_NAME
   # WEBAPP_NAME must be globally unique (e.g. pse-genesis-yourname123)
   ```

2. **Log in and deploy:**

   ```bash
   az login
   az account set --subscription "<your-subscription-id>"
   chmod +x app-service.sh
   ./app-service.sh all
   ```

3. **Open the app:** `https://<WEBAPP_NAME>.azurewebsites.net`

**Commands:**

- `./app-service.sh all` – build image, push to ACR, create App Service resources (if missing), deploy
- `./app-service.sh create` – only create resource group, ACR, App Service plan, and Web App
- `./app-service.sh deploy` – build, push, and update the Web App to the new image

---

## Option 3: Azure AKS (Kubernetes)

1. **Copy env example and set your values:**

   ```bash
   cd deploy/azure
   cp .env.example .env
   # Edit .env: AZURE_SUBSCRIPTION_ID, RESOURCE_GROUP_NAME, AKS_CLUSTER_NAME,
   # ACR_NAME, and optionally AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID
   ```

2. **Log in to Azure:**

   ```bash
   az login
   az account set --subscription "<your-subscription-id>"
   ```

3. **Create ACR and attach to AKS (if you don’t have one yet):**

   ```bash
   export ACR_NAME=<your-acr-name>   # unique name
   az acr create --resource-group $RESOURCE_GROUP_NAME --name $ACR_NAME --sku Basic
   az aks update --resource-group $RESOURCE_GROUP_NAME --name $AKS_CLUSTER_NAME --attach-acr $ACR_NAME
   ```

   Then set `ACR_NAME=<your-acr-name>` in your `.env`.

4. **Deploy:**

   ```bash
   chmod +x deploy.sh
   ./deploy.sh all
   ```

   This will:
   - Build the Genesis image (Go server + Vue frontend)
   - Push it to your ACR
   - Apply the Kubernetes manifests and set the deployment image

## Options

- `./deploy.sh build` – only build the Docker image
- `./deploy.sh push` – build and push to ACR (requires `ACR_NAME` in `.env`)
- `./deploy.sh apply` – only apply Kubernetes manifests (assumes image already in cluster or ACR)
- `./deploy.sh all` – build, push, and apply

## After deploy

- **Pods:** `kubectl get pods -n hangar-genesis`
- **Service:** `kubectl get svc -n hangar-genesis`
- **Ingress:** The manifest uses a default host (e.g. `genesis.<your-ip>.sslip.io`). Ensure your ingress controller (e.g. NGINX) is installed and that the host points to the cluster. Set `GENESIS_INGRESS_HOST` in `.env` or edit the manifest to match your host.

## Rancher URL and token

The Rancher URL and token you have are for Rancher itself (e.g. managing clusters). The Genesis app does not require them to run. If you later want the Genesis UI to talk to Rancher, that would be a separate integration (e.g. env vars in the Deployment); for this deploy we only need Azure/AKS and optional ACR.

## Container Registry (ACR) quick reference

The Azure portal “Get started” steps for your registry map to the following. The deploy scripts (`container-app.sh`, `app-service.sh`, `deploy.sh`) do steps 2–4 for the **Genesis image** automatically.

1. **Install Docker** – [Docker for Mac / Windows / Linux](https://docs.docker.com/get-docker/).

2. **Log in to your registry** (Azure CLI + Docker must be installed):
   ```bash
   az login
   az acr login --name <ACR_NAME>
   ```
   Example: `az acr login --name genesisacr123`

3. **Build and tag the Genesis image** (or pull/tag any image):
   ```bash
   # Script does: docker build, then tag as <ACR_NAME>.azurecr.io/hangar-genesis:latest
   docker tag hangar-genesis:latest <ACR_NAME>.azurecr.io/hangar-genesis:latest
   ```

4. **Push the image**:
   ```bash
   docker push <ACR_NAME>.azurecr.io/hangar-genesis:latest
   ```
   Example: `docker push genesisacr123.azurecr.io/hangar-genesis:latest`

To do steps 2–4 in one go, run `./container-app.sh all` (or `./app-service.sh all` / `./deploy.sh all`) with `ACR_NAME` set in `.env`.

## Variables reference

| Variable | Description |
|---------|-------------|
| `AZURE_SUBSCRIPTION_ID` | Azure subscription ID |
| `AZURE_LOCATION` | e.g. `westeurope` |
| `RESOURCE_GROUP_NAME` | Resource group (used by both AKS and App Service) |
| `ACR_NAME` | Azure Container Registry name (required for App Service; required for AKS push) |
| **Container Apps** | |
| `CONTAINERAPP_NAME` | Container app name (default: `genesis-app`) |
| `CONTAINERAPP_ENV` | Container Apps environment name (default: `genesis-env`) |
| **App Service** | |
| `APP_SERVICE_PLAN_NAME` | App Service plan name (default: `genesis-plan`) |
| `WEBAPP_NAME` | Web App name, globally unique (e.g. `pse-genesis-xyz`) |
| **AKS** | |
| `AKS_CLUSTER_NAME` | AKS cluster name |
| `AZURE_CLIENT_ID` | Service principal app ID (for `az aks get-credentials` if needed) |
| `AZURE_CLIENT_SECRET` | Service principal secret |
| `AZURE_TENANT_ID` | Azure AD tenant ID |
| `GENESIS_INGRESS_HOST` | Override ingress host (e.g. `genesis.<your-ip>.sslip.io`) |
