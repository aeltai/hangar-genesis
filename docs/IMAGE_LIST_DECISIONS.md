# How the image list is decided (what we show the customer to mirror)

This document explains **what data we use** and **how we decide** which images to show the customer for their air-gap setup.

---

## 1. Inputs (what we fetch)

We build the list from these **sources**:

| Source | What it is | Where it comes from |
|--------|------------|----------------------|
| **Charts** | Helm chart repos (Rancher, Fleet, addons, etc.) | **Community:** `https://github.com/rancher/charts` (branch e.g. `release-v2.13`). **Rancher Prime:** same GitHub repo today; Prime also offers charts at `charts.rancher.com` (different/extra versions like `-ent`), but image discovery in hangar uses the GitHub clone. |
| **KDM** | Kontainer Driver Metadata (K3s/RKE2/RKE1 versions, system images) | **Same for both:** `https://releases.rancher.com/kontainer-driver-metadata/<branch>/data.json` (e.g. `release-v2.13`). |
| **Application Collection** (optional) | Charts + container images from Rancher Application Collection | `https://api.apps.rancher.io` + registry `dp.apps.rancher.io`. |

So for **Rancher Prime**, the main thing that changes for the customer is **where they get the Rancher Helm chart and which Rancher versions are available** (e.g. `rancher-prime` = charts.rancher.com with -ent builds). For **generating the image list**, we currently use the same chart content (GitHub) and the same KDM (releases.rancher.com) for both Community and Prime.

---

## 2. Decision flow (how we decide what images to show)

High level:

1. **Rancher version** – You choose a Rancher version (e.g. `v2.13.1`). This drives chart branch and KDM branch (e.g. `release-v2.13`).
2. **Source (Community vs Rancher Prime)** – Only affects where we *say* charts/KDM come from and which product you’re targeting; image discovery uses the same URLs above.
3. **Step 2 choices** – Distros (K3s, RKE2, RKE1), CNI, load balancer, Linux/Windows, and **which Kubernetes versions** to include. These restrict which KDM-derived images (distro, CNI, LB) we include.
4. **Step 3 choices** – Groups: **Essentials** (Rancher core + Fleet + CNI + distro + LB), **AddOns** (monitoring, logging, storage, etc.), **Application Collection** (if enabled). You can expand and select subgroups/charts/images. Only selected items contribute to the list.
5. **Filtering** – We filter the full discovered set by:
   - Selected cluster types and Kubernetes versions (from Step 2)
   - Selected CNI (only that CNI’s images)
   - Selected load balancer options (K3s/RKE2 LB images)
   - Linux-only vs Linux+Windows (Step 2)
   - Selected groups/charts/images (Step 3)

So the **decisions** that determine the final list are:

- **Rancher version** → chart branch + KDM branch.
- **Source (Community / Rancher Prime)** → same chart/KDM sources for image discovery; Prime changes product/version availability and where you pull the Rancher chart (charts.rancher.com), not how we build the list.
- **Step 2:** distros, CNI, LB, Linux/Windows, **Kubernetes versions** → which KDM-based images (K3s/RKE2/RKE1 core, CNI, LB) are included.
- **Step 3:** Essentials vs AddOns vs Application Collection, and within them which subgroups/charts/images are selected → which chart-based and app-collection images are included.

---

## 3. Where each type of image comes from

| Image type | Decided by | Data source |
|------------|------------|-------------|
| **Rancher / Fleet / system charts** | Chart repo + branch (from Rancher version); group “Essentials” | Charts (GitHub rancher/charts) |
| **K3s/RKE2/RKE1 core** | Step 2: distros + selected K8s versions | KDM data.json → k3s-images.txt, rke2-images*.txt, etc. |
| **CNI** | Step 2: CNI choice | KDM + chart images for that CNI only |
| **Load balancer (Klipper, Traefik, NGINX)** | Step 2: LB toggles | KDM + chart images |
| **AddOns (monitoring, logging, storage, …)** | Step 3: AddOns group and selected subgroups/charts | Charts (same repo, addon charts) |
| **Application Collection** | Step 3: Application Collection + Charts/Containers | api.apps.rancher.io + dp.apps.rancher.io |

---

## 4. Rancher Prime vs Community (what actually changes)

- **Availability of Rancher version** – Prime may offer additional or different Rancher versions (e.g. -ent) via `charts.rancher.com` and your `rancher-prime` Helm repo. We don’t change Rancher version availability in code; you pick the version (e.g. v2.13.1) and we use the same KDM + chart branch logic.
- **Rancher Helm chart source** – With Prime you’d typically use `charts.rancher.com` (rancher-prime) to *install* Rancher; with Community you use `releases.rancher.com` (rancher-stable) or GitHub. For **generating the image list**, we still use the GitHub chart repo to discover images; the list is then valid for mirroring whether you install from Community or Prime chart repos.
- **KDM** – Same URL for both (releases.rancher.com). No China/GC mirror in use for Prime in this flow.

So: **decisions** are “Rancher version + Source + Step 2 (distros, CNI, LB, K8s versions, platform) + Step 3 (groups/charts/images)”. **Data** we use to decide what to show is: chart repos (GitHub), KDM (releases.rancher.com), and optionally Application Collection API.
