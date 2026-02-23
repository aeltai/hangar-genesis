<script setup lang="ts">
import { ref, reactive, onMounted, watch, onUnmounted } from 'vue'
import { fetchRancherVersions, fetchStep1OptionsMerged, generate, exportImageList, fetchLogs, type RancherVersionInfo } from './api/genesis'
import type { Step1OptionsResponse, GenerateRequest, GenerateResponse } from './types/genesis'
import Step1Form from './components/Step1Form.vue'
import Step3Tree from './components/Step3Tree.vue'

const VERSION = '0.1'
const theme = ref<'dark' | 'light'>('dark')

onMounted(async () => {
  const saved = localStorage.getItem('genesis-theme') as 'dark' | 'light' | null
  if (saved) theme.value = saved
  if (theme.value === 'light') document.documentElement.setAttribute('data-theme', 'light')
})

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  if (theme.value === 'light') {
    document.documentElement.setAttribute('data-theme', 'light')
  } else {
    document.documentElement.removeAttribute('data-theme')
  }
  localStorage.setItem('genesis-theme', theme.value)
}

const step = ref<'step1' | 'loading' | 'step3'>('step1')
// Hash-based page: '' = app, 'docs' = documentation (separate page)
const page = ref<'app' | 'docs'>(
  typeof window !== 'undefined' && window.location.hash.slice(1) === 'docs' ? 'docs' : 'app'
)
function updatePageFromHash() {
  page.value = window.location.hash.slice(1) === 'docs' ? 'docs' : 'app'
}
function goBackToApp() {
  window.location.hash = ''
  updatePageFromHash()
}
onMounted(() => {
  updatePageFromHash()
  window.addEventListener('hashchange', updatePageFromHash)
})
onUnmounted(() => {
  window.removeEventListener('hashchange', updatePageFromHash)
})
const step1Options = ref<Step1OptionsResponse | null>(null)
const rancherVersions = ref<RancherVersionInfo[]>([])
const step1Error = ref('')
const includeRC = ref(false)
const includeGitHubVersions = ref(false)
const genRequest = reactive<GenerateRequest>({
  rancherVersion: 'v2.13.1',
  rancherVersions: ['v2.13.1'],
  isRPMGC: false,
  includeAppCollectionCharts: false,
  appCollectionAPIUser: '',
  appCollectionAPIPassword: '',
  distros: ['rke2'],
  cni: 'cni_calico',
  loadBalancer: true,
  lbK3sKlipper: false,
  lbK3sTraefik: false,
  lbRKE2Nginx: true,
  lbRKE2Traefik: false,
  includeWindows: false,
  k3sVersions: ['all'],
  rke2Versions: ['all'],
  rkeVersions: ['all'],
  destinationRegistry: '',
  destinationRegistryUser: '',
  destinationRegistryPassword: '',
})

const genResponse = ref<GenerateResponse | null>(null)
const genError = ref('')
const exportError = ref('')
const showLogs = ref(false)
const serverLogs = ref<string[]>([])
const logsContentRef = ref<HTMLElement | null>(null)
let logsPollTimer: ReturnType<typeof setInterval> | null = null

function startLogsPoll() {
  if (logsPollTimer) return
  async function poll() {
    try {
      serverLogs.value = await fetchLogs()
    } catch {
      // ignore
    }
  }
  poll()
  logsPollTimer = setInterval(poll, 1500)
}

function stopLogsPoll() {
  if (logsPollTimer) {
    clearInterval(logsPollTimer)
    logsPollTimer = null
  }
}

watch(
  () => [step.value, showLogs.value] as const,
  ([s, show]) => {
    if (s === 'loading' && show) startLogsPoll()
    else stopLogsPoll()
  }
)
watch(serverLogs, () => {
  if (logsContentRef.value) logsContentRef.value.scrollTop = logsContentRef.value.scrollHeight
})
onUnmounted(stopLogsPoll)

const optionsLoading = ref(false)
let loadAbort: AbortController | null = null

async function loadRancherVersions() {
  try {
    rancherVersions.value = await fetchRancherVersions(includeRC.value)
  } catch { /* ignore */ }
}

async function loadOptions() {
  if (loadAbort) loadAbort.abort()
  loadAbort = new AbortController()
  step1Error.value = ''
  optionsLoading.value = true
  const versions = genRequest.rancherVersions?.length ? genRequest.rancherVersions : [genRequest.rancherVersion]
  try {
    step1Options.value = await fetchStep1OptionsMerged(versions, includeRC.value, includeGitHubVersions.value)
  } catch (e) {
    if ((e as Error).name === 'AbortError') return
    step1Error.value = e instanceof Error ? e.message : String(e)
  } finally {
    optionsLoading.value = false
  }
}

watch(() => [genRequest.rancherVersion, genRequest.rancherVersions], () => { loadOptions() }, { deep: true })
watch(includeRC, async () => {
  await loadRancherVersions()
  loadOptions()
})
watch(includeGitHubVersions, () => { loadOptions() })

onMounted(async () => {
  await loadRancherVersions()
  loadOptions()
})

async function runGenerate() {
  genError.value = ''
  const versions = genRequest.rancherVersions?.length ? genRequest.rancherVersions : (genRequest.rancherVersion ? [genRequest.rancherVersion] : [])
  if (!versions.length) {
    genError.value = 'Select at least one Rancher version.'
    return
  }
  step.value = 'loading'
  genRequest.loadBalancer = genRequest.lbK3sKlipper || genRequest.lbK3sTraefik || genRequest.lbRKE2Nginx || genRequest.lbRKE2Traefik
  try {
    genResponse.value = await generate(genRequest)
    step.value = 'step3'
  } catch (e) {
    genError.value = e instanceof Error ? e.message : String(e)
    step.value = 'step1'
  }
}

async function runExport(
  selectedComponentIDs: string[],
  chartNames: string[],
  selectedImageRefs: string[]
) {
  if (!genResponse.value) return
  exportError.value = ''
  try {
    const blob = await exportImageList({
      jobId: genResponse.value.jobId,
      selectedComponentIDs,
      chartNames,
      selectedImageRefs,
    })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'images.txt'
    a.click()
    URL.revokeObjectURL(a.href)
  } catch (e) {
    exportError.value = e instanceof Error ? e.message : String(e)
  }
}

function backToStep1() {
  step.value = 'step1'
  genResponse.value = null
}
</script>

<template>
  <div class="app">
    <template v-if="page === 'docs'">
      <div class="docs-page">
        <div class="docs-page-header">
          <a href="#" class="docs-back" @click.prevent="goBackToApp">← Back to Genesis</a>
          <h1 class="docs-page-title">Documentation</h1>
        </div>
        <div class="docs-panel">
          <div class="docs-body">
            <h3>Overview</h3>
            <p>
              <strong>Hangar Genesis</strong> generates image lists for Rancher air-gapped deployments.
              It fetches metadata from multiple sources to build a comprehensive, customizable list of
              container images and Helm charts needed for your specific deployment.
            </p>

            <h3>Step 1: Source &amp; Options</h3>
            <table class="docs-table">
              <tr><td><strong>Rancher version</strong></td><td>Select the target Rancher Manager version. Versions are fetched from <a href="https://github.com/rancher/rancher/tags" target="_blank">GitHub tags</a>. Enable "Include pre-release" for RC/alpha/beta versions.</td></tr>
              <tr><td><strong>Image list source</strong></td><td><em>Community</em> fetches K3s/RKE2 image lists from GitHub releases. <em>Rancher Prime</em> fetches from <code>prime.ribs.rancher.io</code> (curated/certified lists, includes <code>rancher-images.txt</code>). Both use the same KDM and chart repos.</td></tr>
              <tr><td><strong>Application Collection</strong></td><td>Optionally include charts from <code>dp.apps.rancher.io</code> (Rancher Application Collection marketplace). Requires API credentials.</td></tr>
              <tr><td><strong>Distros</strong></td><td>Select Kubernetes distributions: <em>K3s</em>, <em>RKE2</em>, and/or <em>RKE1</em> (legacy, Rancher &lt;2.12 only).</td></tr>
              <tr><td><strong>Kubernetes versions</strong></td><td>
                Versions from two sources:<br/>
                &bull; <strong>KDM</strong> (Kontainer Driver Metadata) &mdash; officially supported by this Rancher release, fetched from <code>releases.rancher.com</code><br/>
                &bull; <strong>GitHub</strong> <span style="opacity:0.7">[GH]</span> &mdash; newer releases from <a href="https://github.com/rancher/rke2/releases" target="_blank">rancher/rke2</a> / <a href="https://github.com/k3s-io/k3s/releases" target="_blank">k3s-io/k3s</a> not yet in KDM<br/>
                Select "All" for KDM versions, or pick specific versions.
              </td></tr>
              <tr><td><strong>Platform</strong></td><td><em>Linux only</em> or <em>Linux + Windows</em> (adds Windows node images for RKE2/K3s).</td></tr>
              <tr><td><strong>CNI</strong></td><td>Container Network Interface plugin: Canal, Calico, Cilium, Flannel (K3s only), or All.</td></tr>
              <tr><td><strong>Load Balancer / Ingress</strong></td><td>K3s: Klipper (ServiceLB) and/or Traefik. RKE2: NGINX Ingress and/or Traefik.</td></tr>
            </table>

            <h3>Step 2: Generate</h3>
            <p>The backend fetches data from these sources:</p>
            <ul>
              <li><strong>KDM</strong> &mdash; <code>releases.rancher.com/kontainer-driver-metadata/</code> for compatible K8s versions and their system images</li>
              <li><strong>Chart repositories</strong> &mdash; Rancher charts (<code>charts.rancher.io</code> or <code>charts.rancher.com</code>) for Helm chart images</li>
              <li><strong>GitHub Releases API</strong> &mdash; RKE2/K3s releases for additional version information and release notes</li>
              <li><strong>Application Collection API</strong> &mdash; <code>api.apps.rancher.io</code> for marketplace charts (if enabled)</li>
            </ul>

            <h3>Step 3: Groups &amp; Charts</h3>
            <p>The generated tree has two main groups:</p>
            <ul>
              <li><strong>Essentials</strong> &mdash; Core Rancher components, selected distro images, CNI plugin, load balancer/ingress. Pre-selected by default.</li>
              <li><strong>AddOns</strong> &mdash; Optional charts grouped by category: Monitoring, Logging, Backup, Storage, Security, CIS, Provisioning (cloud operators), Cluster API, OS Management, Support.</li>
            </ul>
            <p>Each group shows charts and their container images. You can toggle individual charts or images. The preview panels show:</p>
            <ul>
              <li><strong>Charts</strong> &mdash; with legend tags: [R] Rancher, [C] CNI, [RKE2] distro, [LB] ingress, [A] addon</li>
              <li><strong>Images</strong> &mdash; with group tags and origin chart name. Use "Check Availability" to verify images exist in their registries.</li>
            </ul>

            <h3>Export</h3>
            <p>"Export image list" downloads a <code>images.txt</code> file with one image reference per line, ready for use with <code>hangar mirror</code>, <code>hangar save</code>, or other air-gap tooling.</p>
            <p>Optionally set a <strong>destination registry</strong> in Step 3 to see copy-paste commands for mirroring (<code>hangar mirror -f images.txt -d &lt;registry&gt;</code>), saving/loading a zip bundle, or using <a href="https://github.com/rancher/hauler" target="_blank" rel="noopener noreferrer">Hauler</a> to create a store or mirror.</p>

            <h3>Release Notes</h3>
            <p>When specific RKE2/K3s versions are selected (not "All"), a "Release Notes &amp; Chart Versions" section appears showing changelog and bundled chart versions from the GitHub release page.</p>
          </div>
        </div>
      </div>
    </template>

    <template v-else>
    <header class="hero">
      <div class="hero-inner">
        <div class="hero-brand">
          <h1 class="hero-title">Genesis</h1>
          <span class="hero-version">v{{ VERSION }}</span>
        </div>
        <div class="hero-actions">
          <a href="https://github.com/aeltai/hangar-genesis" target="_blank" rel="noopener noreferrer" class="hero-link" title="GitHub Repository">
            <svg viewBox="0 0 16 16" width="18" height="18" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
            GitHub
          </a>
          <a href="#docs" class="hero-link" title="Documentation">Docs</a>
          <button type="button" class="theme-toggle" :title="theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'" @click="toggleTheme">
            {{ theme === 'dark' ? 'Light' : 'Dark' }}
          </button>
        </div>
      </div>
      <p class="hero-subtitle">
        Build image lists for air-gapped Rancher: choose Rancher version(s), distros (K3s, RKE2, RKE1), CNI, load balancer, and charts—then export one list to mirror or save.
      </p>
    </header>
    <main class="main">
      <div v-if="step === 'step1'" class="panel step1-panel">
        <div class="step1-layout">
          <div class="step1-form">
            <Step1Form
              :available-rancher-versions="rancherVersions"
              v-model:rancher-version="genRequest.rancherVersion"
              v-model:rancher-versions="genRequest.rancherVersions"
              v-model:include-r-c="includeRC"
              v-model:include-git-hub-versions="includeGitHubVersions"
              v-model:is-rpm-gc="genRequest.isRPMGC"
              v-model:include-app-collection="genRequest.includeAppCollectionCharts"
              v-model:app-user="genRequest.appCollectionAPIUser"
              v-model:app-password="genRequest.appCollectionAPIPassword"
              v-model:distros="genRequest.distros"
              v-model:cni="genRequest.cni"
              v-model:lb-k3s-klipper="genRequest.lbK3sKlipper"
              v-model:lb-k3s-traefik="genRequest.lbK3sTraefik"
              v-model:lb-rke2-nginx="genRequest.lbRKE2Nginx"
              v-model:lb-rke2-traefik="genRequest.lbRKE2Traefik"
              v-model:include-windows="genRequest.includeWindows"
              v-model:k3s-versions="genRequest.k3sVersions"
              v-model:rke2-versions="genRequest.rke2Versions"
              v-model:rke-versions="genRequest.rkeVersions"
              :options="step1Options"
              :load-error="step1Error"
              :options-loading="optionsLoading"
              @generate="runGenerate"
            />
            <p v-if="genError" class="error">{{ genError }}</p>
          </div>
          <aside v-if="(genRequest.rancherVersions?.length || genRequest.rancherVersion) && step1Options" class="step1-details">
            <h3 class="details-title">Details for this configuration</h3>
            <div class="details-section">
              <h4 class="details-heading">Rancher</h4>
              <ul class="details-links">
                <li v-for="v in (genRequest.rancherVersions?.length ? genRequest.rancherVersions : [genRequest.rancherVersion])" :key="v">
                  <a :href="'https://github.com/rancher/rancher/releases/tag/' + v" target="_blank" rel="noopener noreferrer">Release {{ v }}</a>
                </li>
                <li><a href="https://ranchermanager.docs.rancher.com/releases" target="_blank" rel="noopener noreferrer">Rancher release notes</a></li>
              </ul>
            </div>
            <div class="details-section">
              <h4 class="details-heading">Lifecycle &amp; support</h4>
              <ul class="details-links">
                <li><a href="https://www.suse.com/lifecycle" target="_blank" rel="noopener noreferrer">SUSE Product Lifecycle</a></li>
                <li><a href="https://www.suse.com/suse-rancher/support-matrix/all-supported-versions" target="_blank" rel="noopener noreferrer">Rancher support matrix (all versions)</a></li>
                <li v-for="v in (genRequest.rancherVersions?.length ? genRequest.rancherVersions : [genRequest.rancherVersion])" :key="'matrix-' + v">
                  <a :href="'https://www.suse.com/suse-rancher/support-matrix/all-supported-versions/rancher-v' + v.replace(/^v/, '').replace(/\./g, '-')" target="_blank" rel="noopener noreferrer">Support matrix {{ v }}</a>
                </li>
              </ul>
            </div>
            <div v-if="step1Options.details" class="details-section">
              <h4 class="details-heading">Data sources</h4>
              <dl class="details-dl">
                <dt>KDM</dt>
                <dd><a v-if="step1Options.details.kdmUrl" :href="step1Options.details.kdmUrl" target="_blank" rel="noopener noreferrer">KDM data</a></dd>
                <dt>Image lists</dt>
                <dd><code>{{ step1Options.details.imageListSource }}</code></dd>
                <dt>Charts</dt>
                <dd><code>rancher/charts (release-v{{ (genRequest.rancherVersions?.[0] || genRequest.rancherVersion)?.replace(/^v/,'').split('.').slice(0,2).join('.') }})</code></dd>
              </dl>
            </div>
            <div class="details-section">
              <h4 class="details-heading">Distro docs</h4>
              <ul class="details-links">
                <li v-if="genRequest.distros.includes('k3s')"><a href="https://docs.k3s.io/" target="_blank" rel="noopener noreferrer">K3s</a></li>
                <li v-if="genRequest.distros.includes('rke2')"><a href="https://docs.rke2.io/" target="_blank" rel="noopener noreferrer">RKE2</a></li>
                <li v-if="genRequest.distros.includes('rke')"><a href="https://rke.docs.rancher.com/" target="_blank" rel="noopener noreferrer">RKE1</a></li>
              </ul>
            </div>
          </aside>
        </div>
      </div>

      <div v-else-if="step === 'loading'" class="panel loading-panel">
        <p class="loading-text">Generating tree from KDM and charts…</p>
        <p class="loading-hint">This may take a minute.</p>
        <div class="loading-logs">
          <button type="button" class="logs-toggle" @click="showLogs = !showLogs">
            {{ showLogs ? 'Hide logs' : 'Show logs' }}
          </button>
          <div v-show="showLogs" class="logs-viewer">
            <pre ref="logsContentRef" class="logs-content">{{ serverLogs.length ? serverLogs.join('\n') : 'Waiting for server logs…' }}</pre>
          </div>
        </div>
      </div>

      <div v-else-if="step === 'step3' && genResponse" class="panel panel-fullscreen">
        <Step3Tree
          :job-id="genResponse.jobId"
          :roots="genResponse.roots"
          :basic-charts="genResponse.basicCharts"
          :basic-image-component="genResponse.basicImageComponent"
          :past-selection="genResponse.pastSelection"
          :components="genRequest.distros.join(',')"
          :cni-for-standard="genRequest.cni"
          :rancher-versions="genRequest.rancherVersions?.length ? genRequest.rancherVersions : (genRequest.rancherVersion ? [genRequest.rancherVersion] : [])"
          :rke2-versions="genRequest.distros.includes('rke2') ? genRequest.rke2Versions : []"
          :k3s-versions="genRequest.distros.includes('k3s') ? genRequest.k3sVersions : []"
          v-model:destination-registry="genRequest.destinationRegistry"
          v-model:destination-registry-user="genRequest.destinationRegistryUser"
          v-model:destination-registry-password="genRequest.destinationRegistryPassword"
          @export-list="runExport"
          @back="backToStep1"
        />
        <p v-if="exportError" class="error">{{ exportError }}</p>
      </div>
    </main>

    <footer class="footer">
      <a href="https://github.com/cnrancher/hangar" target="_blank" rel="noopener noreferrer">Hangar</a>
      <span class="footer-sep">·</span>
      <a href="https://ranchermanager.docs.rancher.com/" target="_blank" rel="noopener noreferrer">Rancher Manager docs</a>
      <span class="footer-sep">·</span>
      <a href="https://docs.rke2.io/" target="_blank" rel="noopener noreferrer">RKE2 docs</a>
      <span class="footer-sep">·</span>
      <a href="https://docs.k3s.io/" target="_blank" rel="noopener noreferrer">K3s docs</a>
      <span class="footer-sep">·</span>
      <a href="https://github.com/aeltai" target="_blank" rel="noopener noreferrer">@aeltai</a>
    </footer>
    </template>
  </div>
</template>

<style scoped>
.app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg);
  color: var(--text);
}
.hero {
  padding: 1.25rem 2rem 1.5rem;
  border-bottom: 1px solid var(--border);
  background: linear-gradient(135deg, var(--panel) 0%, color-mix(in srgb, var(--panel) 95%, var(--border)) 100%);
  position: relative;
}
.hero::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 4px;
  background: linear-gradient(180deg, var(--cyan), var(--green));
  border-radius: 0 2px 2px 0;
}
.hero-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1.5rem;
  flex-wrap: wrap;
  margin-bottom: 0.75rem;
}
.hero-brand {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}
.hero-title {
  font-size: 2rem;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: var(--cyan);
  margin: 0;
  text-shadow: 0 0 20px color-mix(in srgb, var(--cyan) 30%, transparent);
}
.hero-version {
  font-size: 0.8rem;
  font-weight: 600;
  opacity: 0.85;
  color: var(--text);
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  background: color-mix(in srgb, var(--border) 40%, transparent);
}
.hero-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.hero-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0.4rem 0.75rem;
  font-size: 0.85rem;
  font-weight: 600;
  border-radius: 8px;
  background: color-mix(in srgb, var(--bg) 80%, var(--border));
  border: 1px solid var(--border);
  color: var(--text);
  text-decoration: none;
  cursor: pointer;
  transition: border-color 0.2s, color 0.2s;
}
.hero-link:hover {
  border-color: var(--cyan);
  color: var(--cyan);
}
.theme-toggle {
  padding: 0.4rem 0.75rem;
  font-size: 0.85rem;
  font-weight: 600;
  border-radius: 8px;
  background: color-mix(in srgb, var(--bg) 80%, var(--border));
  border: 1px solid var(--border);
  color: var(--text);
  cursor: pointer;
  transition: border-color 0.2s;
}
.theme-toggle:hover {
  border-color: var(--cyan);
}
.docs-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  padding: 0 2rem 2rem;
}
.docs-page-header {
  padding: 1.5rem 0 1rem;
  border-bottom: 1px solid var(--border);
  margin-bottom: 1.5rem;
}
.docs-back {
  display: inline-block;
  font-size: 0.9rem;
  color: var(--cyan);
  text-decoration: none;
  margin-bottom: 0.75rem;
}
.docs-back:hover {
  text-decoration: underline;
}
.docs-page-title {
  margin: 0;
  font-size: 1.75rem;
  font-weight: 700;
  color: var(--cyan);
}
.docs-panel {
  max-width: 900px;
  margin: 0 auto;
  padding: 0;
  flex: 1;
}
.docs-body {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  font-size: 0.9rem;
  line-height: 1.6;
}
.docs-body h3 {
  color: var(--green, #22c55e);
  margin: 1.25rem 0 0.5rem;
  font-size: 1rem;
}
.docs-body h3:first-child {
  margin-top: 0;
}
.docs-body p {
  margin: 0.5rem 0;
}
.docs-body ul {
  margin: 0.5rem 0;
  padding-left: 1.25rem;
}
.docs-body li {
  margin: 0.25rem 0;
}
.docs-body code {
  background: var(--bg);
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 0.85em;
}
.docs-body a {
  color: var(--cyan);
}
.docs-table {
  width: 100%;
  border-collapse: collapse;
  margin: 0.5rem 0;
}
.docs-table td {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
}
.docs-table td:first-child {
  white-space: nowrap;
  width: 180px;
}
.hero-subtitle {
  margin: 0;
  opacity: 0.88;
  font-size: 0.9rem;
  letter-spacing: 0.01em;
  max-width: 56rem;
}
.main {
  flex: 1;
  padding: 1.5rem 2rem;
  max-width: 1400px;
  margin: 0 auto;
  width: 100%;
}
.main:has(.panel-fullscreen) {
  max-width: 100%;
  padding: 0.5rem 1rem;
}
.panel {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
}
.step1-panel {
  padding: 0;
}
.step1-layout {
  display: grid;
  grid-template-columns: 1fr minmax(280px, 340px);
  gap: 1.5rem;
  align-items: start;
}
.step1-form {
  min-width: 0;
  padding: 1.5rem;
}
.step1-details {
  padding: 1.25rem 1.5rem;
  background: color-mix(in srgb, var(--bg) 60%, var(--panel));
  border-left: 1px solid var(--border);
  border-radius: 0 8px 8px 0;
  position: sticky;
  top: 1rem;
}
.details-title {
  font-size: 1rem;
  color: var(--cyan);
  margin: 0 0 1rem 0;
}
.details-section {
  margin-bottom: 1.25rem;
}
.details-heading {
  font-size: 0.8rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text);
  opacity: 0.9;
  margin: 0 0 0.5rem 0;
}
.details-links {
  list-style: none;
  margin: 0;
  padding: 0;
}
.details-links li {
  margin-bottom: 0.35rem;
}
.details-links a {
  font-size: 0.9rem;
}
.details-dl {
  margin: 0;
  font-size: 0.85rem;
}
.details-dl dt {
  font-weight: 600;
  margin-top: 0.5rem;
  color: var(--text);
  opacity: 0.9;
}
.details-dl dt:first-child {
  margin-top: 0;
}
.details-dl dd {
  margin: 0.2rem 0 0 0;
}
.details-dl code {
  font-size: 0.8rem;
  word-break: break-all;
}
@media (max-width: 900px) {
  .step1-layout {
    grid-template-columns: 1fr;
  }
  .step1-details {
    position: static;
    border-left: none;
    border-top: 1px solid var(--border);
    border-radius: 0 0 8px 8px;
  }
}
.panel-fullscreen {
  min-height: calc(100vh - 160px);
  display: flex;
  flex-direction: column;
}
.loading-panel {
  text-align: center;
  padding: 3rem;
}
.loading-text {
  font-size: 1.1rem;
  margin-bottom: 0.5rem;
}
.loading-hint {
  opacity: 0.7;
  font-size: 0.9rem;
}
.loading-logs {
  margin-top: 1.5rem;
  text-align: left;
  max-width: 900px;
  margin-left: auto;
  margin-right: auto;
}
.logs-toggle {
  margin-bottom: 0.5rem;
}
.logs-viewer {
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  overflow: hidden;
}
.logs-content {
  margin: 0;
  padding: 0.75rem 1rem;
  font-size: 0.8rem;
  line-height: 1.4;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 320px;
  overflow: auto;
}
.error {
  color: var(--red);
  margin-top: 1rem;
  font-size: 0.9rem;
}
.footer {
  margin-top: auto;
  padding: 1rem 2rem;
  border-top: 1px solid var(--border);
  background: var(--panel);
  font-size: 0.85rem;
  text-align: center;
  color: var(--text);
  opacity: 0.9;
}
.footer a {
  color: var(--cyan);
  text-decoration: none;
}
.footer a:hover {
  text-decoration: underline;
}
.footer-sep {
  margin: 0 0.5rem;
  opacity: 0.6;
}

/* Mobile & tablet */
@media (max-width: 768px) {
  .hero {
    padding: 1rem 1rem 1.25rem;
  }
  .hero-title {
    font-size: 1.5rem;
  }
  .hero-actions {
    width: 100%;
    justify-content: flex-start;
  }
  .hero-subtitle {
    font-size: 0.85rem;
  }
  .main {
    padding: 1rem;
  }
  .main:has(.panel-fullscreen) {
    padding: 0.5rem 0.75rem;
  }
  .panel {
    padding: 1rem;
  }
  .step1-form {
    padding: 1rem;
  }
  .step1-details {
    padding: 1rem 1.25rem;
  }
  .docs-page {
    padding: 0 1rem 1.5rem;
  }
  .docs-body {
    padding: 1rem;
  }
  .docs-table td:first-child {
    width: 120px;
  }
  .footer {
    padding: 0.75rem 1rem;
    font-size: 0.8rem;
  }
  .footer-sep {
    margin: 0 0.35rem;
  }
  .loading-panel {
    padding: 2rem 1rem;
  }
  .logs-content {
    max-height: 240px;
  }
}

@media (max-width: 480px) {
  .hero {
    padding: 0.75rem 0.75rem 1rem;
  }
  .hero-inner {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.75rem;
  }
  .hero-title {
    font-size: 1.35rem;
  }
  .hero-actions {
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .hero-link,
  .theme-toggle {
    padding: 0.35rem 0.6rem;
    font-size: 0.8rem;
  }
  .hero-subtitle {
    font-size: 0.8rem;
  }
  .main {
    padding: 0.75rem;
  }
  .main:has(.panel-fullscreen) {
    padding: 0.35rem 0.5rem;
  }
  .panel {
    padding: 0.75rem;
    border-radius: 6px;
  }
  .step1-form {
    padding: 0.75rem;
  }
  .step1-details {
    padding: 0.75rem 1rem;
  }
  .docs-page {
    padding: 0 0.75rem 1rem;
  }
  .docs-page-title {
    font-size: 1.4rem;
  }
  .docs-body {
    padding: 0.75rem;
    font-size: 0.85rem;
  }
  .docs-table {
    font-size: 0.85rem;
  }
  .docs-table td {
    padding: 0.4rem 0.5rem;
    display: block;
  }
  .docs-table td:first-child {
    width: 100%;
    font-weight: 600;
    padding-bottom: 0.15rem;
  }
  .docs-table tr {
    display: block;
    margin-bottom: 0.75rem;
    padding-bottom: 0.75rem;
    border-bottom: 1px solid var(--border);
  }
  .docs-table tr:last-child {
    border-bottom: none;
  }
  .footer {
    padding: 0.5rem 0.75rem;
    font-size: 0.75rem;
  }
  .footer a {
    display: inline-block;
    margin: 0.1rem 0;
  }
  .footer-sep {
    display: inline;
  }
  .loading-panel {
    padding: 1.5rem 0.75rem;
  }
  .loading-text {
    font-size: 1rem;
  }
  .logs-content {
    max-height: 180px;
    font-size: 0.75rem;
  }
}
</style>
