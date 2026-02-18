# Hangar Genesis – Vue 3 frontend

Web UI for **Hangar Genesis**, matching the same flow as the terminal TUI:

1. **Step 1** – Rancher version, source (Community / Rancher Prime), Application Collection, distros (K3s, RKE2, RKE1), CNI, load balancer options, platform (Linux / Linux+Windows), and Kubernetes versions per distro.
2. **Generate** – Calls the backend to run the generator and build the group/chart tree.
3. **Step 3** – Tree of Essentials (Basic) and AddOns: expand/collapse, toggle groups/charts/images, preview charts and images, then **Export** to download the image list.

## Run with the API server

1. Start the Genesis API server (from the repo root):

   ```bash
   go run main.go genesis serve --port=8080 --static=./frontend/dist
   ```

   This serves the API at `/api/*` and the built frontend at `/`.

2. Open [http://localhost:8080](http://localhost:8080).

## Development

1. Start the API server (no static files):

   ```bash
   go run main.go genesis serve --port=8080
   ```

2. Start the Vite dev server (proxies `/api` to the Go server):

   ```bash
   cd frontend && npm run dev
   ```

3. Open the URL shown by Vite (e.g. http://localhost:5173).

## Build

```bash
cd frontend && npm install && npm run build
```

Output is in `frontend/dist`. Use `--static=./frontend/dist` when running `genesis serve` to serve it.
