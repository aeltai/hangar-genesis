# Hangar Genesis

Generate image lists for Rancher air-gapped deployments (Community and Prime). Use the web UI to pick Rancher version(s), distros (K3s, RKE2, RKE1), CNI, load balancer, and charts—then export a single list to mirror or bundle with **[Hangar](https://github.com/cnrancher/hangar)** or **[Hauler](https://github.com/rancher/hauler)**.

## Live demo

**[Try Genesis →](https://genesis-app.wonderfulsea-dc99daa3.westeurope.azurecontainerapps.io/)**

Build your list in the browser: select options, toggle groups/charts/images, optionally set a destination registry, then download `images.txt` and use it with Hangar (mirror, save/load) or Hauler (store, mirror).

## What Genesis does

- Fetches Rancher, K3s, RKE2, and RKE1 version data from KDM and GitHub
- Lets you choose distros, CNI, load balancer/ingress, and optional add-on charts
- Outputs one image list (and chart list) you can export as `images.txt`
- Optional: scan selected images (Trivy) and check image availability

The exported list is a plain text file (one image per line), ready for:

- **[Hangar](https://github.com/cnrancher/hangar)** — `hangar mirror`, `hangar save` / `hangar load`, `hangar scan`
- **[Hauler](https://github.com/rancher/hauler)** — create a store (zip) or mirror into your registry

See the in-app **Docs** (including the **API reference** for HTTP endpoints and pipelines) and the “Next steps” section (when you set a destination registry) for exact commands.

## Run locally

**Backend (API + serves frontend):**

```bash
# Build frontend
cd frontend && npm ci && npm run build && cd ..

# Run Genesis server (serves API + static frontend)
go run main.go genesis serve --port=8080 --static=./frontend/dist
```

Open http://localhost:8080

**Optional:** Set `GITHUB_TOKEN` or `GITHUB_PAT` (GitHub Personal Access Token) to avoid API rate limits when fetching Rancher/K3s/RKE2 versions and release notes.

## Deploy (Azure Container Apps)

See [deploy/azure/README.md](deploy/azure/README.md). Use [container-app.sh](deploy/azure/container-app.sh) to build, push, and deploy. Set `GITHUB_TOKEN` in `deploy/azure/.env` so the container app uses it for GitHub API calls.

## Keeping in sync with Hangar

This repo is based on **[Hangar](https://github.com/cnrancher/hangar)** (SUSE Rancher). We ship Hangar’s code plus the Genesis server and Vue UI. To get upstream fixes and features (mirror, save/load, scan, etc.), merge from Hangar periodically.

**Do we regularly merge?** Yes. Pull in upstream when you want to stay current (e.g. after a Hangar release or when you need a fix).

**How to merge upstream:**

```bash
git fetch upstream
git merge upstream/main
```

(Use `upstream` = `https://github.com/cnrancher/hangar.git`; add it with `git remote add upstream ...` if needed.)

- Resolve conflicts if any (often in `pkg/`, `main.go`, or shared commands). Genesis-specific code lives under `pkg/commands/genesis*.go`, `genesis_serve.go`, and `frontend/`.
- Run tests and try the Genesis UI after merging.
- Optionally note the Hangar version you merged (e.g. in a release note or a “Based on Hangar v1.x” line in this README) so you know what you’re on next time.

## License

Copyright 2025 SUSE Rancher

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
