# Frontend source files (for isolating issues)

All paths are relative to `frontend/`.

## Entry & app

- **src/main.ts** – Vue app entry
- **src/App.vue** – Root component: step flow, genRequest/genResponse, Step1Form + Step3Tree

## Step 3 (Groups & charts – chart column, gecko, flag)

- **src/components/Step3Tree.vue** – Step 3 UI: tree, chart/image preview, character sheet, gecko with flag

Chart column logic lives in **Step3Tree.vue**:

- `selectedBasicImageRefs()` – image refs from selected Essentials subgroups only (basic_*)
- `previewCharts` – computed that:
  - when **Essentials (basic)** is selected: collects charts by recursing into selected nodes (including basic_charts)
  - when **only some Essentials subgroups** are selected (e.g. Rancher): calls `collectFromBasicCharts(root, allowedRefs)` and only adds a chart if at least one of its images is in `allowedRefs`
- So deselecting e.g. Rancher removes Rancher’s images from `allowedRefs` → charts that only have those images disappear from the column.

## Step 1 & API / types

- **src/components/Step1Form.vue** – Step 1 form (version, distros, CNI, etc.)
- **src/api/genesis.ts** – API client (fetchStep1Options, generate, export, scan, logs)
- **src/types/genesis.ts** – Types (TreeNode, GenerateRequest, GenerateResponse, etc.)

## Other

- **src/style.css** – Global styles
- **src/content/genesis-documentation.md** – Docs content
- **src/components/HelloWorld.vue**, **ProductInstallInstructions.vue** – Optional components

## Quick open (from repo root)

```bash
# Step 3 + chart column + gecko + flag
frontend/src/components/Step3Tree.vue

# App (passes rancherVersion, versions to Step3Tree)
frontend/src/App.vue
```
