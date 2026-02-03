<script setup lang="ts">
import { ref, reactive, onMounted, watch, onUnmounted } from 'vue'
import { fetchRancherVersions, fetchStep1Options, generate, exportImageList, fetchLogs } from './api/genesis'
import type { Step1OptionsResponse, GenerateRequest, GenerateResponse } from './types/genesis'
import Step1Form from './components/Step1Form.vue'
import Step3Tree from './components/Step3Tree.vue'

const VERSION = '0.1'
const theme = ref<'dark' | 'light'>('dark')

onMounted(() => {
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
const step1Options = ref<Step1OptionsResponse | null>(null)
const rancherVersions = ref<string[]>([])
const step1Error = ref('')
const genRequest = reactive<GenerateRequest>({
  rancherVersion: 'v2.13.1',
  isRPMGC: false,
  includeAppCollectionCharts: false,
  appCollectionAPIUser: '',
  appCollectionAPIPassword: '',
  distros: ['rke2'],
  cni: 'cni_calico',
  loadBalancer: true,
  lbK3sKlipper: true,
  lbK3sTraefik: true,
  lbRKE2Nginx: true,
  lbRKE2Traefik: true,
  includeWindows: false,
  k3sVersions: 'all',
  rke2Versions: 'all',
  rkeVersions: 'all',
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

async function loadOptions() {
  step1Error.value = ''
  try {
    if (rancherVersions.value.length === 0) {
      rancherVersions.value = await fetchRancherVersions()
    }
    step1Options.value = await fetchStep1Options(genRequest.rancherVersion)
  } catch (e) {
    step1Error.value = e instanceof Error ? e.message : String(e)
  }
}

async function runGenerate() {
  genError.value = ''
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
    <header class="hero">
      <div class="hero-brand">
        <img
          v-if="theme === 'dark'"
          src="/genesis-logo-black.png"
          alt="Genesis"
          class="hero-logo"
        />
        <img
          v-else
          src="/genesis-logo.svg"
          alt="Genesis"
          class="hero-logo"
        />
        <div class="hero-titles">
          <h1 class="hero-title">Hangar Genesis</h1>
          <span class="hero-version">v{{ VERSION }}</span>
        </div>
        <button type="button" class="theme-toggle" :title="theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'" @click="toggleTheme">
          {{ theme === 'dark' ? 'Light' : 'Dark' }}
        </button>
      </div>
      <p class="hero-subtitle">
        Generate image lists for Rancher air-gapped deployments. Select distros, CNI, load balancer, and groups/charts.
      </p>
    </header>

    <main class="main">
      <div v-if="step === 'step1'" class="panel">
        <Step1Form
          :rancher-versions="rancherVersions"
          v-model:rancher-version="genRequest.rancherVersion"
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
          @load-options="loadOptions"
          @generate="runGenerate"
        />
        <p v-if="genError" class="error">{{ genError }}</p>
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

      <div v-else-if="step === 'step3' && genResponse" class="panel">
        <Step3Tree
          :roots="genResponse.roots"
          :basic-charts="genResponse.basicCharts"
          :basic-image-component="genResponse.basicImageComponent"
          :past-selection="genResponse.pastSelection"
          :components="genRequest.distros.join(',')"
          :cni-for-standard="genRequest.cni"
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
  padding: 1rem 2rem 1.25rem;
  border-bottom: 1px solid var(--border);
  background: var(--panel);
}
.hero-brand {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}
.hero-logo {
  height: 48px;
  width: auto;
  display: block;
}
.hero-titles {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  flex: 1;
}
.hero-title {
  font-size: 1.75rem;
  font-weight: 700;
  color: var(--cyan);
  margin: 0;
}
.hero-version {
  font-size: 0.85rem;
  opacity: 0.8;
  color: var(--text);
}
.theme-toggle {
  padding: 0.35rem 0.65rem;
  font-size: 0.85rem;
  font-weight: 500;
  border-radius: 6px;
  background: var(--bg);
  border: 1px solid var(--border);
  color: var(--text);
  cursor: pointer;
}
.theme-toggle:hover {
  border-color: var(--cyan);
}
.hero-subtitle {
  margin: 0;
  opacity: 0.9;
  font-size: 0.95rem;
}
.main {
  flex: 1;
  padding: 1.5rem 2rem;
  max-width: 1400px;
  margin: 0 auto;
  width: 100%;
}
.panel {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
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
</style>
