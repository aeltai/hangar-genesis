# Hangar Genesis — Documentation

**Hangar Genesis** is a tool that uses [Hangar](https://github.com/cnrancher/hangar) to generate **modular Rancher / RKE2 / K3s image lists** for air-gapped deployments. You can then **load, save, and transfer** those image lists into air-gapped environments using **[Hauler](https://docs.hauler.dev/docs/intro)** (the “Airgap Swiss Army Knife”).

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

- **Community (GitHub):** images use `**docker.io/rancher/...`** (e.g. `docker.io/rancher/hardened-calico:...`).
- **Prime:** images use `**registry.rancher.com/rancher/...`** (e.g. `registry.rancher.com/rancher/hardened-calico:...`).

**Rancher core (Prime only):** `rancher-images.txt` uses short form `rancher/...` (i.e. **docker.io** when used).

So the only registry difference is **RKE2**: Prime’s RKE2 list points to **registry.rancher.com**; Community’s to **docker.io**. K3s and rancher core stay on **docker.io** in both.

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

We treat a version as compatible for Rancher `v2.13.1` when `v2.13.1` is between min and max (inclusive). That list is what we show as “RKE2 versions” in the UI.

**KDM (Prime GC, China):** **[https://charts.rancher.cn/kontainer-driver-metadata](https://charts.rancher.cn/kontainer-driver-metadata)** — same branch pattern (`release-v2.13/data.json`).

**Example: Rancher v2.13.1 + RKE2 v1.32.11+rke2r3**


| What                      | Community URL                                                                                                                                                                                              | Prime URL                                                                                                                                                                    |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| KDM                       | [https://releases.rancher.com/kontainer-driver-metadata/release-v2.13/data.json](https://releases.rancher.com/kontainer-driver-metadata/release-v2.13/data.json)                                           | (same or charts.rancher.cn)                                                                                                                                                  |
| RKE2 image list (Linux)   | [https://github.com/rancher/rke2/releases/download/v1.32.11%2Brke2r3/rke2-images-all.linux-amd64.txt](https://github.com/rancher/rke2/releases/download/v1.32.11%2Brke2r3/rke2-images-all.linux-amd64.txt) | [https://prime.ribs.rancher.io/rke2/v1.32.11%2Brke2r3/rke2-images-all.linux-amd64.txt](https://prime.ribs.rancher.io/rke2/v1.32.11%2Brke2r3/rke2-images-all.linux-amd64.txt) |
| RKE2 image list (Windows) | [https://github.com/rancher/rke2/releases/download/v1.32.11%2Brke2r3/rke2-images.windows-amd64.txt](https://github.com/rancher/rke2/releases/download/v1.32.11%2Brke2r3/rke2-images.windows-amd64.txt)     | [https://prime.ribs.rancher.io/rke2/v1.32.11%2Brke2r3/rke2-images.windows-amd64.txt](https://prime.ribs.rancher.io/rke2/v1.32.11%2Brke2r3/rke2-images.windows-amd64.txt)     |
| Rancher core images       | (from charts)                                                                                                                                                                                              | [https://prime.ribs.rancher.io/rancher/v2.13.1/rancher-images.txt](https://prime.ribs.rancher.io/rancher/v2.13.1/rancher-images.txt)                                         |


**K3s example (v1.32.11+k3s3):**


| What           | Community URL                                                                                                                                                    | Prime URL                                                                                                                            |
| -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| K3s image list | [https://github.com/k3s-io/k3s/releases/download/v1.32.11%2Bk3s3/k3s-images.txt](https://github.com/k3s-io/k3s/releases/download/v1.32.11%2Bk3s3/k3s-images.txt) | [https://prime.ribs.rancher.io/k3s/v1.32.11%2Bk3s3/k3s-images.txt](https://prime.ribs.rancher.io/k3s/v1.32.11%2Bk3s3/k3s-images.txt) |


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
| **[https://github.com/rancher/charts](https://github.com/rancher/charts)**               | Addon charts (monitoring, logging, etc.) | `release-v2.13` or `dev-v2.14` (for alpha/beta) |
| **[https://github.com/rancher/system-charts](https://github.com/rancher/system-charts)** | System charts (rancher-monitoring, etc.) | Same                                            |


Branch is chosen from the Rancher version you selected: e.g. Rancher `v2.13.1` → branch `release-v2.13`. We clone that branch and build an index of all chart versions in the repo.

**How we pick which chart version to use:**

Each chart **version** declares which Rancher versions it supports in one of two ways:

1. **Chart.yaml** — annotation:
  `catalog.cattle.io/rancher-version: ">= 2.6.0 < 2.7.0"` (semver constraint).
2. **questions.yaml** (in the chart) — fields:
  `rancher_min_version`, `rancher_max_version` (e.g. `"2.6.3"` and `"2.6.4"`).

We compare your selected Rancher version (e.g. `v2.13.1`) against that constraint. Only chart versions that **satisfy** the constraint are included. For most charts we then take the **latest** such version (index is sorted newest first). For a few (e.g. **rancher-monitoring** in system-charts), we include **all** matching versions so airgap can serve multiple Rancher lines.

**Example:**

- You choose Rancher **v2.13.1**.
- We use **rancher/charts** branch **release-v2.13** and **rancher/system-charts** branch **release-v2.13**.
- For each addon (e.g. `rancher-monitoring`), we read the index, filter versions by `catalog.cattle.io/rancher-version` (or questions.yaml), keep only those where `v2.13.1` satisfies the constraint, and use the latest (or all for monitoring).
- Those chart versions drive the images we add to the tree and export.

**Links:**

- **rancher/charts (branches):** [https://github.com/rancher/charts/branches](https://github.com/rancher/charts/branches) (e.g. `release-v2.13`, `dev-v2.14`).
- **rancher/system-charts (branches):** [https://github.com/rancher/system-charts/branches](https://github.com/rancher/system-charts/branches).
- Example chart with annotation: in any branch, open a chart (e.g. `charts/rancher-monitoring`) and check `Chart.yaml` for `annotations.catalog.cattle.io/rancher-version` or the chart’s `questions.yaml` for `rancher_min_version` / `rancher_max_version`.

So addon chart versions per Rancher version = **same branch as Rancher major.minor** + **filter chart versions by rancher-version constraint**.

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
# Interactive: prompts for Step 1, then shows tree and selection (Step 2/3)
hangar genesis --rancher=v2.13.1 --tui
```

- **Required:** `--rancher=<version>` (e.g. `v2.13.1`). Optional: `--output=images.txt`, `--registry=<dest-registry>`.
- **Step 1:** Choose distros (k3s, rke2, rke), CNI, load balancer, Windows, then pick K3s/RKE2/RKE versions (or “all”) from the list.
- **Step 2/3:** Generator runs; then the tree is shown. Select/deselect nodes (Basic, Addons, charts). Export writes the image list to the output file.
- **Save your choices as YAML:** run with `--save-config=my-config.yaml`. After you finish the TUI (export), the current selections are written to `my-config.yaml`. You can then re-run non-interactively with `--config=my-config.yaml`.
- **Exit:** `q` or Ctrl+C exits (Ctrl+C exits immediately without saving).

```bash
# TUI and save the resulting config for later CI use
hangar genesis --rancher=v2.13.1 --tui --save-config=genesis-config.yaml
```

### YAML config (non-interactive)

Run Genesis without any prompts by passing a YAML config file. Same options as the TUI and web UI; ideal for CI, scripts, and repeatable runs.

```bash
# Non-interactive: all options come from the YAML file
hangar genesis --rancher=v2.13.1 --config=genesis-config.yaml
```

- **Required:** `--rancher=<version>` and `--config=<path>`.
- **Optional flags:** `--output=images.txt`, `--registry=<dest-registry>`, `--rke2-images=...`, etc. Output defaults to `<rancher-version>-images.txt` if not set.
- **Where to get the YAML:**  
  - **Export YAML** from the web UI (Step 3), or  
  - **TUI** with `--save-config=file.yaml` after export, or  
  - Start from the example in the repo: **`generate-list-config.example.yaml`** (see [YAML Config Format](#yaml-config-format) for fields).

The config file contains distros, CNI, loadBalancer, versions (per distro), groups (e.g. `basic`, `addon_monitoring`), optional `charts` and `destinationRegistry` / auth, and optional `scan` settings. No prompts are shown (except overwrite if the output file already exists).

### Summary

| Mode        | Command / source                    | Use case                          |
| ----------- | ----------------------------------- | --------------------------------- |
| **Web UI**  | Browser + Genesis server            | Visual flow, pipelines (API)      |
| **TUI**     | `hangar genesis --rancher=X --tui`  | Terminal, one-off or --save-config |
| **YAML**    | `hangar genesis --rancher=X --config=file.yaml` | CI, scripts, repeat runs  |

---

## YAML Config Format

Genesis can be run from the CLI with a YAML config (no UI). The same format is produced when you **Export YAML** from the app or when you use **--save-config** after a TUI run.

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


### Example YAML

```yaml
# Example: hangar genesis --rancher=v2.13.1 --config=config.yaml

distros:
  - rke2
  # - k3s
  # - rke

sourceType: community   # or prime-gc

cni: cni_calico

loadBalancer: true
# includeWindows: false

# versions (optional; use "all" in UI for all)
versions:
  rke2:
    - v1.34.3+rke2r1
  # k3s:
  #   - v1.28.5
  # rke:
  #   - v1.28.15

groups:
  - basic
  - addon_monitoring
  - addon_logging
  # - addons
  # - addon_storage
  # - addon_security
  # - addon_backup-restore
  # - app_collection

# selectedProducts:
#   - k3k

# Optional: vulnerability scan
scan:
  enabled: true
  jobs: 1
  timeout: 10m
  report: ""
```

### Running from CLI

```bash
# With config file (non-interactive)
hangar genesis --rancher=v2.13.1 --config=genesis-config.yaml

# Optional: custom output paths
hangar genesis --rancher=v2.13.1 --config=genesis-config.yaml \
  --output=my-images.txt \
  --rke2-images=rke2-images.txt
```

**Example config in repo:** The repository includes **`generate-list-config.example.yaml`** at the project root with commented options (distros, CNI, versions, groups, destination registry, scan). Copy and edit it for your run or use it as reference for all supported keys (including `rancherVersions`, `includeRC`, `includeGitHubVersions`, `lbK3sKlipper`, `lbRKE2Nginx`, etc.).

---

## API reference

The Genesis server exposes HTTP APIs used by the UI. You can call them from scripts or pipelines (e.g. curl, GitHub Actions). All endpoints are under `/api/`. CORS allows `*` for GET and POST.

**Base URL:** same origin as the app (e.g. `https://your-genesis-host` or `http://localhost:8080`).

### Generate and export flow


| Step | Method | Endpoint                               | Description                                                                                                                                                              |
| ---- | ------ | -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1    | GET    | `/api/rancher-versions`                | List available Rancher versions. Optional query: `includeRC=true`.                                                                                                       |
| 2    | GET    | `/api/step1-options?rancher=<version>` | Step 1 options (KDM capabilities, K3s/RKE2/RKE versions). Optional: `includeRC=true`, `includeGitHubVersions=true`.                                                      |
| 3    | POST   | `/api/generate`                        | Start a generate job. Body: JSON (rancherVersion, distros, cni, k3sVersions, rke2Versions, rkeVersions, etc.). Returns `jobId`, `roots`, `basicCharts`, `pastSelection`. |
| 4    | POST   | `/api/export`                          | Export image list for a job. Body: `{ "jobId", "selectedComponentIDs", "chartNames", "selectedImageRefs" }`. Returns `images.txt` (text/plain).                          |


- **Job lifetime:** Jobs expire after 60 minutes.
- **k3sVersions / rke2Versions / rkeVersions:** Send as comma-separated strings (e.g. `"v1.32.11+k3s3"`), not arrays. Use versions from `/api/step1-options` for compatibility.

### Public GET endpoints (pipelines / automation)

After you have a `jobId` from `POST /api/generate` and have run `POST /api/export` at least once for that job:


| Method | Endpoint                             | Description                                                                                                                                            |
| ------ | ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| GET    | `/api/genesis/image-list?jobId=<id>` | Returns the exported image list (same as `images.txt`). Re-fetch without sending a POST body. **Requires:** job must have been exported at least once. |
| GET    | `/api/genesis/config?jobId=<id>`     | Returns the genesis list config as YAML (same as `--save-config`). No export required.                                                                 |


Example (replace `BASE` and `JOB_ID`):

```bash
curl -o images.txt "${BASE}/api/genesis/image-list?jobId=${JOB_ID}"
curl -o genesis-config.yaml "${BASE}/api/genesis/config?jobId=${JOB_ID}"
```

### Registry auth (backend-generated auth file)


| Method | Endpoint                     | Description                                                                                                                                                                                                                                                 |
| ------ | ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| POST   | `/api/genesis/registry-auth` | Body: `{ "destinationRegistry", "destinationRegistryUser", "destinationRegistryPassword" }`. Returns a Docker/containers-style auth JSON (attachment `auth.json`). Use with `REGISTRY_AUTH_FILE` so Hangar can push without running `hangar login` locally. |


### Other endpoints


| Method | Endpoint                                         | Description                                                                                        |
| ------ | ------------------------------------------------ | -------------------------------------------------------------------------------------------------- |
| POST   | `/api/check-availability`                        | Body: `{ "images": ["ref1", ...] }`. Returns per-image availability (e.g. pullable from registry). |
| POST   | `/api/scan`                                      | Body: `{ "images": ["ref1", ...] }`. Starts Trivy scan; returns `scanJobId`.                       |
| GET    | `/api/scan/status/<id>`                          | Scan job status.                                                                                   |
| GET    | `/api/scan/report/<id>`                          | Download scan report (CSV).                                                                        |
| GET    | `/api/release-notes?repo=<owner/repo>&tag=<tag>` | Fetch GitHub release notes.                                                                        |
| GET    | `/api/logs`                                      | Recent server log lines (e.g. during generate).                                                    |


---

## Using the Image List with Hauler

1. **Generate your image list** in this app (or via `hangar genesis --config=...`) and download the image list file (e.g. `images.txt`).
2. **Create a Hauler store** and add the images from your list (Hauler can read the same image-list format):
  ```bash
   hauler store add-images images.txt
  ```
   Or add images one-by-one; see [Hauler Store](https://docs.hauler.dev/docs/usage/store).
3. **Package the store** (e.g. tar or OCI) and move it to your air-gapped environment.
4. **In the air-gapped environment**, use Hauler to serve the store or load images into your local registry:
  ```bash
   hauler store serve
   # or load images into a registry from the store
  ```

See [Hauler documentation](https://docs.hauler.dev/docs/intro) for full workflows (charts, files, airgap).

---

## Summary


| Item                | Description                                                                                                                           |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| **This app**        | Hangar Genesis UI: configure distros/versions/options → generate tree → select components → export image list or YAML, optional scan. |
| **Hangar**          | Generates and works with image lists: copy, save, load, mirror, sign, scan.                                                           |
| **Hauler**          | Packages and serves image lists (and charts/files) for air-gap: store, serve, load.                                                   |
| **Download Hauler** | [Hauler releases (GitHub)](https://github.com/hauler-dev/hauler/releases)                                                             |


For more on Hangar (CLI, save/load/mirror), see [Hangar Documentation](https://hangar.cnrancher.com/docs/).