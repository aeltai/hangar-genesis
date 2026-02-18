# Hangar Genesis — Documentation

**Hangar Genesis** is a tool that uses [Hangar](https://github.com/cnrancher/hangar) to generate **modular Rancher / RKE2 / K3s image lists** for air-gapped deployments. You can then **load, save, and transfer** those image lists into air-gapped environments using **[Hauler](https://docs.hauler.dev/docs/intro)** (the “Airgap Swiss Army Knife”).

This app provides a web UI and an optional CLI; both produce the same image lists and YAML configs.

---

## Download Hauler

Use Hauler to package and serve your image lists (and charts/files) in air-gapped environments.

| Platform | Download |
|----------|----------|
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

| Option | Description |
|--------|-------------|
| **Rancher version** | e.g. `v2.13.1`. Drives KDM and chart compatibility. |
| **Source** | Community (GitHub + releases.rancher.com) or Rancher Prime (Prime catalog). |
| **Distros** | `k3s`, `rke2`, `rke` (RKE1). One or more. |
| **CNI** | Canal, Calico, Cilium, or Flannel (Flannel only for K3s). |
| **Load balancer** | Include K3s Klipper/Traefik and RKE2 NGINX/Traefik images in Basic (on/off). |
| **Windows** | Include Windows node images for RKE2/K3s (on/off). |
| **K3s / RKE2 / RKE versions** | `all` or a comma-separated list of versions. |
| **Application Collection** | Optional: include charts/images from `dp.apps.rancher.io` (requires API credentials). |
| **Products** | Optional: e.g. **K3K** — fetch Helm chart and add its images to the tree. |

### Step 2 — Generate

- Fetches KDM data and chart metadata, builds the component tree (Basic, Addons, App Collection, products).
- Shows progress and logs; when done, the tree is shown in Step 3.

### Step 3 — Tree and Export

- **Tree:** Expand/collapse components (Rancher, Fleet, K3s/RKE2/RKE, CNI, addon charts, product charts). Select/deselect nodes; selection drives the exported image list.
- **Export image list:** Download a text file with one image reference per line. Use this with Hangar and Hauler.
- **Export YAML:** Download a Genesis config file that mirrors your Step 1 + Step 2 selections (distros, CNI, groups, charts, etc.). Use with the CLI: `hangar genesis --rancher=<ver> --config=<file>`.
- **Scan:** Run Trivy on the currently selected images; download the vulnerability report when finished.

---

## YAML Config Format

Genesis can be run from the CLI with a YAML config (no UI). The same format is produced when you **Export YAML** from the app.

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `distros` | `[]string` | `["k3s", "rke2", "rke"]` — at least one. |
| `sourceType` | `string` | `community` (default) or `prime-gc`. |
| `cni` | `string` | e.g. `cni_calico`, `cni_canal`, `cni_flannel`, `cni_cilium`, or `cni` (all). |
| `loadBalancer` | `bool` | Include LB/ingress images in Basic. |
| `includeWindows` | `bool` | Include Windows node images. |
| `includeAppCollectionCharts` | `bool` | Include charts from Application Collection. |
| `versions` | `map[string][]string` | Per-distro versions, e.g. `rke2: ["v1.34.3+rke2r1"]`. Omit or use `all` in UI for all. |
| `groups` | `[]string` | e.g. `basic`, `addons`, `addon_monitoring`, `addon_logging`, `app_collection`. |
| `charts` | `[]string` | Optional: specific chart names (overrides groups). |
| `selectedProducts` | `[]string` | e.g. `["k3k"]`. |
| `scan` | `object` | Optional: `enabled`, `jobs`, `timeout`, `report`. |

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

| Item | Description |
|------|-------------|
| **This app** | Hangar Genesis UI: configure distros/versions/options → generate tree → select components → export image list or YAML, optional scan. |
| **Hangar** | Generates and works with image lists: copy, save, load, mirror, sign, scan. |
| **Hauler** | Packages and serves image lists (and charts/files) for air-gap: store, serve, load. |
| **Download Hauler** | [Hauler releases (GitHub)](https://github.com/hauler-dev/hauler/releases) |

For more on Hangar (CLI, save/load/mirror), see [Hangar Documentation](https://hangar.cnrancher.com/docs/).
