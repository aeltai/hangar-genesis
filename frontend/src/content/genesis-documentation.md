<!-- Synced from README.md — edit README.md only. -->

# Hangar Genesis

**Fork of [Hangar](https://github.com/cnrancher/hangar)** (SUSE Rancher) — this repo adds the Genesis server and web UI on top of upstream Hangar. For syncing with upstream, see [README-PROJECT.md](README-PROJECT.md).

**Repo:** [github.com/aeltai/hangar-genesis](https://github.com/aeltai/hangar-genesis) · **Live demo:** [Try Genesis →](https://genesis-app.wonderfulsea-dc99daa3.westeurope.azurecontainerapps.io/)

For **run locally, deploy, and upstream sync**, see [README-PROJECT.md](README-PROJECT.md).

---

**Hangar Genesis** is a tool that uses [Hangar](https://github.com/cnrancher/hangar) to generate **modular Rancher / RKE2 / K3s image lists** for air-gapped deployments. You can then **load, save, and transfer** those image lists into air-gapped environments using **[Hauler](https://docs.hauler.dev/docs/intro)** (the "Airgap Swiss Army Knife").

This app provides a web UI and an optional CLI; both produce the same image lists and YAML configs.

---

## Download Hauler

Use Hauler to package and serve your image lists (and charts/files) in air-gapped environments.


| Platform                                 | Download                                                                       |
| ---------------------------------------- | ------------------------------------------------------------------------------ |
| **All releases (Linux, macOS, Windows)** | **[Hauler releases on GitHub](https://github.com/hauler-dev/hauler/releases)** |


Pick the latest release and download the binary for your OS (e.g. `hauler_linux_amd64`, `hauler_darwin_arm64`). No container runtime is required.

---

## What This Tool Does

1. **Step 1 — Configure:** Choose Rancher version, distros (K3s, RKE2, RKE1), CNI, load balancer images, Kubernetes versions, optional products (e.g. K3K), and (optionally) Application Collection charts.
2. **Step 2 — Generate:** Build a component tree from KDM and chart data (Rancher, Fleet, addons, charts).
3. **Step 3 — Select & Export:** In the tree, select the components/charts/images you want, then:
  - **Export image list** — plain list of image references (e.g. `images.txt`) for use with Hangar save/load/mirror and with Hauler.
  - **Export YAML** — save your selections as a Genesis config file so you can re-run the same setup via CLI or CI.
  - **Scan** — optionally run Trivy vulnerability scan on the selected images and download a report.

The generated **image list** is the input for:

- **Hangar** `save` / `load` / `mirror` (copy images to/from registries or archives).
- **Hauler** `store` (add images to a Hauler store), then `serve` or move the store into an air-gapped environment and use Hauler to load images from the store into your registry.

---

## Functionality Overview

### Step 1 — Configuration


| Option                        | Description                                                                           |
| ----------------------------- | ------------------------------------------------------------------------------------- |
| **Rancher version**           | e.g. `v2.13.1`. Drives KDM and chart compatibility.                                   |
| **Source**                    | Community (GitHub + releases.rancher.com) or Rancher Prime (Prime catalog).           |
| **Distros**                   | `k3s`, `rke2`, `rke` (RKE1). One or more.                                             |
| **CNI**                       | Canal, Calico, Cilium, or Flannel (Flannel only for K3s).                             |
| **Load balancer**             | Include K3s Klipper/Traefik and RKE2 NGINX/Traefik images in Basic (on/off).          |
| **Windows**                   | Include Windows node images for RKE2/K3s (on/off).                                    |
| **K3s / RKE2 / RKE versions** | `all` or a comma-separated list of versions.                                          |
| **Application Collection**    | Optional: include charts/images from `dp.apps.rancher.io` (requires API credentials). |
| **Products**                  | Optional: e.g. **K3K** — fetch Helm chart and add its images to the tree.             |


### Prime vs Community: image lists and registries (verified with curl)

Where lists are fetched from:


| Item                    | Community                                                   | Rancher Prime                                                                   |
| ----------------------- | ----------------------------------------------------------- | ------------------------------------------------------------------------------- |
| **KDM**                 | `releases.rancher.com/kontainer-driver-metadata`            | Same.                                                                           |
| **Charts**              | GitHub: `rancher/charts`, `rancher/system-charts`           | Same.                                                                           |
| **Rancher core images** | From charts (no single file).                               | Single list: `https://prime.ribs.rancher.io/rancher/vX.Y.Z/rancher-images.txt`. |
| **K3s image list**      | GitHub: `k3s-io/k3s` → `k3s-images.txt`.                    | `https://prime.ribs.rancher.io/k3s/{version}/k3s-images.txt`.                   |
| **RKE2 image list**     | GitHub: `rancher/rke2` → `rke2-images-all.linux-amd64.txt`. | `https://prime.ribs.rancher.io/rke2/{version}/rke2-images-all.linux-amd64.txt`. |


**K3s:** For a given version, the **image list content is the same** for Community and Prime (same lines, same `docker.io/rancher/...` references). Only the download URL differs.

**RKE2:** Same image names and tags, **different registry in the list**:

- **Community (GitHub):** images use **`docker.io/rancher/...`** (e.g. `docker.io/rancher/hardened-calico:...`).
- **Prime:** images use **`registry.rancher.com/rancher/...`** (e.g. `registry.rancher.com/rancher/hardened-calico:...`).

**Rancher core (Prime only):** `rancher-images.txt` uses short form `rancher/...` (i.e. **docker.io** when used).

So the only registry difference is **RKE2**: Prime's RKE2 list points to **registry.rancher.com**; Community's to **docker.io**. K3s and rancher core stay on **docker.io** in both.

### Repos and URLs we use

Single reference for all repos and base URLs used by Genesis (Community vs Prime / Prime GC).

| Purpose | Community | Rancher Prime | Prime GC (China) |
| ------- | --------- | ------------- | ----------------- |
| **Charts (addons)** | [github.com/rancher/charts](https://github.com/rancher/charts) | Same | [github.com/cnrancher/pandaria-catalog](https://github.com/cnrancher/pandaria-catalog) |
| **System charts** | [github.com/rancher/system-charts](https://github.com/rancher/system-charts) | Same | [github.com/cnrancher/system-charts](https://github.com/cnrancher/system-charts) |
| **KDM (version metadata)** | [releases.rancher.com/kontainer-driver-metadata](https://releases.rancher.com/kontainer-driver-metadata) | Same | [charts.rancher.cn/kontainer-driver-metadata](https://charts.rancher.cn/kontainer-driver-metadata) |
| **Image lists (K3s, RKE2, rancher-images)** | GitHub releases (k3s-io/k3s, rancher/rke2) + charts | [prime.ribs.rancher.io](https://prime.ribs.rancher.io) | — |
| **Application Collection (optional)** | [api.apps.rancher.io/v1](https://api.apps.rancher.io/v1), charts OCI: `dp.apps.rancher.io`, containers: `dp.apps.rancher.io/containers` | Same | Same |
| **K3s/RKE2 image list mirror (CN)** | [rancher-mirror.rancher.cn](https://rancher-mirror.rancher.cn) (k3s, rke2) | — | — |

- Chart repos use branches `release-v2.13` or `dev-v2.14` (by Rancher major.minor).
- KDM uses the same branch pattern; data file e.g. `.../release-v2.13/data.json`.
- Rancher versions list in the UI: [api.github.com/repos/rancher/rancher/releases](https://api.github.com/repos/rancher/rancher/releases).

### Example: KDM data and image list URLs

Genesis gets **RKE2 (and K3s) versions** from **KDM** (Kontainer Driver Metadata). You pick a Rancher version; we load the matching KDM branch and filter distro versions by `minChannelServerVersion` / `maxChannelServerVersion`. Then we fetch image lists only for the version(s) you choose.

**KDM data (Community):**

- Base: **[https://releases.rancher.com/kontainer-driver-metadata](https://releases.rancher.com/kontainer-driver-metadata)**
- Branch by Rancher major.minor: `release-v2.13` or `dev-v2.14` (for alpha/beta/rc).
- Data file: **[https://releases.rancher.com/kontainer-driver-metadata/release-v2.13/data.json](https://releases.rancher.com/kontainer-driver-metadata/release-v2.13/data.json)**

The JSON has top-level keys `k3s`, `rke2`, `rke` (RKE1). Each has a `releases` array. One entry looks like:

```json
{
  "version": "v1.32.11+rke2r3",
  "minChannelServerVersion": "v2.12.0",
  "maxChannelServerVersion": "v2.13.99"
}
```

We treat a version as compatible for Rancher `v2.13.1` when `v2.13.1` is between min and max (inclusive). That list is what we show as "RKE2 versions" in the UI.

**KDM (Prime GC, China):** **[https://charts.rancher.cn/kontainer-driver-metadata](https://charts.rancher.cn/kontainer-driver-metadata)** — same branch pattern (`release-v2.13/data.json`).

**Example: Rancher v2.13.1 + RKE2 v1.32.11+rke2r3**


| What                      | Community URL                                                                                                                                                                                              | Prime URL                                                                                                                                                                    |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| KDM                       | [release-v2.13/data.json](https://releases.rancher.com/kontainer-driver-metadata/release-v2.13/data.json)                                                                                                 | (same or charts.rancher.cn)                                                                                                                                                  |
| RKE2 image list (Linux)   | [rke2 v1.32.11+rke2r3](https://github.com/rancher/rke2/releases/download/v1.32.11%2Brke2r3/rke2-images-all.linux-amd64.txt)                                                                               | [prime.ribs.rancher.io/rke2/...](https://prime.ribs.rancher.io/rke2/v1.32.11%2Brke2r3/rke2-images-all.linux-amd64.txt)                                                      |
| RKE2 image list (Windows) | [GitHub](https://github.com/rancher/rke2/releases/download/v1.32.11%2Brke2r3/rke2-images.windows-amd64.txt)                                                                                              | [prime.ribs.rancher.io](https://prime.ribs.rancher.io/rke2/v1.32.11%2Brke2r3/rke2-images.windows-amd64.txt)                                                                    |
| Rancher core images       | (from charts)                                                                                                                                                                                              | [rancher v2.13.1](https://prime.ribs.rancher.io/rancher/v2.13.1/rancher-images.txt)                                                                                          |


**K3s example (v1.32.11+k3s3):** Community: [GitHub k3s](https://github.com/k3s-io/k3s/releases/download/v1.32.11%2Bk3s3/k3s-images.txt) · Prime: [prime.ribs.rancher.io/k3s/...](https://prime.ribs.rancher.io/k3s/v1.32.11%2Bk3s3/k3s-images.txt)

**Quick check (curl):**

```bash
# RKE2 versions compatible with Rancher v2.13 (from KDM)
curl -sS 'https://releases.rancher.com/kontainer-driver-metadata/release-v2.13/data.json' | jq '.rke2.releases[] | select(.minChannelServerVersion <= "v2.13.99" and .maxChannelServerVersion >= "v2.13.0") | .version' | head -20
```

We only fetch the image list files for the **version(s) you select** (and only the components you select in the tree), so the exported list is a minimal set for that Rancher + RKE2/K3s combo.

### Addon chart versions per Rancher version

Addon charts (Monitoring, Logging, etc.) and system charts come from **GitHub chart repos**. We scope by **Rancher major.minor** and then filter each chart **version** by a Rancher version constraint.

**Chart repos and branches:**


| Repo                                                                                     | Purpose                                  | Branch pattern                                  |
| ---------------------------------------------------------------------------------------- | ---------------------------------------- | ----------------------------------------------- |
| **[github.com/rancher/charts](https://github.com/rancher/charts)**                       | Addon charts (monitoring, logging, etc.) | `release-v2.13` or `dev-v2.14` (for alpha/beta) |
| **[github.com/rancher/system-charts](https://github.com/rancher/system-charts)**         | System charts (rancher-monitoring, etc.) | Same                                            |


Branch is chosen from the Rancher version you selected: e.g. Rancher `v2.13.1` → branch `release-v2.13`. We clone that branch and build an index of all chart versions in the repo.

**How we pick which chart version to use:** Chart **version** declares support via `Chart.yaml` annotation `catalog.cattle.io/rancher-version` (semver constraint) or `questions.yaml` `rancher_min_version` / `rancher_max_version`. We include only versions that match your Rancher version; for most charts we take the **latest** matching version (for some, e.g. rancher-monitoring, we include all for airgap). See [rancher/charts branches](https://github.com/rancher/charts/branches) and [rancher/system-charts branches](https://github.com/rancher/system-charts/branches).

### Step 2 — Generate

- Fetches KDM data and chart metadata, builds the component tree (Basic, Addons, App Collection, products).
- Shows progress and logs; when done, the tree is shown in Step 3.

### Step 3 — Tree and Export

- **Tree:** Expand/collapse components (Rancher, Fleet, K3s/RKE2/RKE, CNI, addon charts, product charts). Select/deselect nodes; selection drives the exported image list.
- **Export image list:** Download a text file with one image reference per line. Use this with Hangar and Hauler.
- **Export YAML:** Download a Genesis config file that mirrors your Step 1 + Step 2 selections (distros, CNI, groups, charts, etc.). Use with the CLI: `hangar genesis --rancher=<ver> --config=<file>`.
- **Scan:** Run Trivy on the currently selected images; download the vulnerability report when finished.

---

## CLI: TUI and YAML config

You can run Genesis in two ways from the command line: **interactive TUI** (`--tui`) or **non-interactive YAML** (`--config`). Both use the same logic as the web UI and produce the same image lists and config format.

### TUI (interactive)

The TUI is a terminal UI that mirrors the web flow: Step 1 (distros, versions, CNI, load balancer, Windows), Step 2 (generate tree), Step 3 (select groups/charts/images in the tree, then export).

```bash
hangar genesis --rancher=v2.13.1 --tui
```

- **Required:** `--rancher=<version>`. Optional: `--output=images.txt`, `--registry=<dest-registry>`.
- **Save choices as YAML:** `--save-config=my-config.yaml` (writes after you export). Then re-run with `--config=my-config.yaml`.
- **Exit:** `q` or Ctrl+C.

### YAML config (non-interactive)

```bash
hangar genesis --rancher=v2.13.1 --config=genesis-config.yaml
```

- **Required:** `--rancher` and `--config`. No prompts (except overwrite).
- **Where to get YAML:** Export YAML from web UI, TUI with `--save-config`, or copy **`generate-list-config.example.yaml`** from the repo.

### Summary

| Mode        | Command / source                    | Use case                          |
| ----------- | ----------------------------------- | --------------------------------- |
| **Web UI**  | Browser + Genesis server            | Visual flow, pipelines (API)      |
| **TUI**     | `hangar genesis --rancher=X --tui`   | Terminal, one-off or --save-config |
| **YAML**    | `hangar genesis --rancher=X --config=file.yaml` | CI, scripts, repeat runs  |

---

## YAML Config Format

Same format as **Export YAML** from the app or **--save-config** after a TUI run.

### Sample configuration

```yaml
# Example: RKE2 + Calico, Basic + Monitoring/Logging, one RKE2 version
distros:
  - rke2
cni: cni_calico
loadBalancer: true

versions:
  rke2:
    - v1.34.3+rke2r1

groups:
  - basic
  - addon_monitoring
  - addon_logging

scan:
  enabled: true
  jobs: 1
  timeout: 10m
```

Use it: `hangar genesis --rancher=v2.13.1 --config=genesis-config.yaml`. Full example with all options: **`generate-list-config.example.yaml`** in the repo root.

### Fields

| Field                        | Type                  | Description                                                                            |
| ---------------------------- | --------------------- | -------------------------------------------------------------------------------------- |
| `distros`                    | `[]string`            | `["k3s", "rke2", "rke"]` — at least one.                                               |
| `sourceType`                 | `string`              | `community` (default) or `prime-gc`.                                                   |
| `cni`                        | `string`              | e.g. `cni_calico`, `cni_canal`, `cni_flannel`, `cni_cilium`, or `cni` (all).           |
| `loadBalancer`               | `bool`                | Include LB/ingress images in Basic.                                                    |
| `includeWindows`             | `bool`                | Include Windows node images.                                                           |
| `includeAppCollectionCharts` | `bool`                | Include charts from Application Collection.                                            |
| `versions`                   | `map[string][]string` | Per-distro versions, e.g. `rke2: ["v1.34.3+rke2r1"]`. Omit or use `all` in UI for all. |
| `groups`                     | `[]string`            | e.g. `basic`, `addons`, `addon_monitoring`, `addon_logging`, `app_collection`.         |
| `charts`                     | `[]string`            | Optional: specific chart names (overrides groups).                                     |
| `selectedProducts`           | `[]string`            | e.g. `["k3k"]`.                                                                        |
| `scan`                       | `object`              | Optional: `enabled`, `jobs`, `timeout`, `report`.                                      |

See **`generate-list-config.example.yaml`** at the project root for all supported keys (including `rancherVersions`, `includeRC`, `includeGitHubVersions`, `destinationRegistry`, per-LB options).

### Running from CLI

```bash
hangar genesis --rancher=v2.13.1 --config=genesis-config.yaml
# Optional: --output=my-images.txt --rke2-images=...
```

---

## Genesis web server

The **Genesis server** is the HTTP service that serves the API and (optionally) the built web UI. Run it with:

```bash
hangar genesis serve [--port=8080] [--static=./frontend/dist]
```

| Flag       | Default   | Description                                                                 |
| ---------- | --------- | --------------------------------------------------------------------------- |
| `--port`   | `8080`    | Port to listen on.                                                          |
| `--static` | *(empty)* | Directory to serve as the frontend (e.g. `./frontend/dist` after building). |

- **With `--static`:** Serves both the API under `/api/` and the UI at `/`. Use this for a single process (e.g. production or "run locally").
- **Without `--static`:** API-only; use when the frontend is served separately (e.g. `npm run dev` in `frontend/` pointing at this server).

Optional: set `GITHUB_TOKEN` or `GITHUB_PAT` so the server can call GitHub without hitting rate limits (Rancher/K3s/RKE2 versions, release notes).

---

## API reference

All endpoints are under `/api/`. Base URL: same origin as the app (e.g. `http://localhost:8080`). CORS allows GET/POST. On error, JSON `{ "error": "<message>" }` with an appropriate status (400, 404, 500, etc.).

### Generate and export flow

| Step | Method | Endpoint                               | Description |
| ---- | ------ | -------------------------------------- | ----------- |
| 1    | GET    | `/api/rancher-versions`                | List available Rancher versions. |
| 2    | GET    | `/api/step1-options?rancher=<version>` | Step 1 options (KDM capabilities, K3s/RKE2/RKE versions). |
| 3    | POST   | `/api/generate`                        | Start a generate job. |
| 4    | POST   | `/api/export`                          | Export image list for a job. |

- **Job lifetime:** Jobs expire after 60 minutes.
- **k3sVersions / rke2Versions / rkeVersions:** Send as comma-separated strings (e.g. `"v1.32.11+k3s3"`), not arrays. Use versions from `/api/step1-options` for compatibility.

#### GET `/api/rancher-versions`

**Query:** `includeRC=true` (optional) — include release candidates.

**Response:** `200 OK`, `application/json`

```json
{
  "versions": [
    { "version": "v2.13.1", "date": "2025-01-15" },
    { "version": "v2.12.5", "date": "2024-11-01" }
  ]
}
```

#### GET `/api/step1-options`

**Query:** `rancher=<version>` (required). Optional: `includeRC=true`, `includeGitHubVersions=true`.

**Response:** `200 OK`, `application/json`

```json
{
  "hasRKE1": false,
  "capabilities": {
    "k3s": {
      "versions": ["v1.32.11+k3s3", "v1.30.5+k3s1"],
      "sources": { "v1.32.11+k3s3": "kdm", "v1.30.5+k3s1": "both" }
    },
    "rke2": {
      "versions": ["v1.32.11+rke2r3", "v1.30.5+rke2r1"],
      "sources": { "v1.32.11+rke2r3": "kdm" }
    }
  },
  "details": {
    "kdmUrl": "https://releases.rancher.com/kontainer-driver-metadata/release-v2.13/data.json",
    "imageListSource": "GitHub (k3s-io/k3s, rancher/rke2)"
  }
}
```

#### POST `/api/generate`

**Body:** JSON with `rancherVersion`, `distros` (array), `cni`, `loadBalancer`, `includeWindows`, `k3sVersions`, `rke2Versions`, `rkeVersions` (strings), optional `rancherVersions`, `isRPMGC`, `includeAppCollectionCharts`, LB flags, etc.

**Response:** `200 OK`, `application/json`

```json
{
  "jobId": "uuid",
  "roots": [
    { "id": "basic", "label": "Basic", "kind": "group", "count": 120, "children": [...] }
  ],
  "basicCharts": [...],
  "basicImageComponent": { "chart-name": "image-ref" },
  "pastSelection": "base64-or-empty"
}
```

Tree nodes: `id`, `label`, `kind` (e.g. `group`, `chart`, `image`), `count`, `children` (optional).

#### POST `/api/export`

**Body:** JSON `{ "jobId", "selectedComponentIDs", "chartNames", "selectedImageRefs" }` (all arrays of strings).

**Response:** `200 OK`, `text/plain`, `Content-Disposition: attachment; filename=images.txt` — body is the image list (one image ref per line). On error: 400/404/500 and JSON `{ "error": "..." }`.

### Public GET endpoints (pipelines / automation)

After you have a `jobId` from `POST /api/generate` and have run `POST /api/export` at least once for that job:

| Method | Endpoint                             | Response |
| ------ | ------------------------------------ | -------- |
| GET    | `/api/genesis/image-list?jobId=<id>` | `200 OK`, `text/plain`, attachment `images.txt` — one image per line. **Requires:** job exported at least once. |
| GET    | `/api/genesis/config?jobId=<id>`     | `200 OK`, `application/x-yaml`, attachment `genesis-config.yaml` — same as `--save-config`. No export required. |

Example (replace `BASE` and `JOB_ID`):

```bash
curl -o images.txt "${BASE}/api/genesis/image-list?jobId=${JOB_ID}"
curl -o genesis-config.yaml "${BASE}/api/genesis/config?jobId=${JOB_ID}"
```

### Registry auth

#### POST `/api/genesis/registry-auth`

**Body:** JSON `{ "destinationRegistry", "destinationRegistryUser", "destinationRegistryPassword" }`.

**Response:** `200 OK`, `application/json`, `Content-Disposition: attachment; filename="auth.json"` — Docker/containers-style auth file for `REGISTRY_AUTH_FILE`.

```json
{
  "auths": {
    "my-registry.example.com": { "auth": "base64(user:password)" },
    "https://my-registry.example.com": { "auth": "base64(user:password)" }
  }
}
```

### Other endpoints

#### POST `/api/check-availability`

**Body:** JSON `{ "images": ["ref1", "ref2", ...] }`.

**Response:** `200 OK`, `application/json` — `{ "results": { "<image>": { "status": "ok"|"not_found"|"error", "detail": "..." } } }`.

#### POST `/api/scan`

**Body:** JSON `{ "images": ["ref1", ...] }` (max 50 images). **Response:** `200 OK`, `application/json` — `{ "scanJobId": "uuid" }`.

#### GET `/api/scan/status/<id>`

**Response:** `200 OK`, `application/json` — `{ "status": "running"|"completed"|"failed", "summary"?: { "critical", "high", "medium", "low" }, "error"?: "..." }`.

#### GET `/api/scan/report/<id>`

**Response:** `200 OK`, `text/csv`, attachment `scan-report.csv` when scan is completed. Otherwise `409 Conflict` and JSON `{ "error": "..." }`.

#### GET `/api/release-notes`

**Query:** `repo=<owner/repo>`, `tag=<tag>` (e.g. `repo=rancher/rancher&tag=v2.13.1`). **Response:** `200 OK`, `application/json` — `{ "tag", "name", "publishedAt", "url", "prerelease", "charts", "changelog", "body" }`.

#### GET `/api/logs`

**Response:** `200 OK`, `application/json` — `{ "lines": [ "log line 1", ... ] }`. Recent server log lines (max 500).

---

## Using the Image List with Hauler

1. Generate your image list (web UI, TUI, or `--config`) and download `images.txt`.
2. **Hauler store:** `hauler store add-images images.txt` (see [Hauler Store](https://docs.hauler.dev/docs/usage/store)).
3. Package the store and move to air-gap; then `hauler store serve` or load into your registry.

See [Hauler documentation](https://docs.hauler.dev/docs/intro).

---

## Summary

| Item                | Description                                                                                                                           |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| **This app**        | Hangar Genesis: configure distros/versions/options → generate tree → select components → export image list or YAML, optional scan.     |
| **Hangar**          | Image lists: copy, save, load, mirror, sign, scan. [Hangar docs](https://hangar.cnrancher.com/docs/).                               |
| **Hauler**          | Air-gap: store, serve, load. [Hauler releases](https://github.com/hauler-dev/hauler/releases)                                         |
