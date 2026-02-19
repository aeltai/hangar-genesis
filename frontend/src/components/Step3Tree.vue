<script setup lang="ts">
import { ref, reactive, computed, watch, onUnmounted } from 'vue'
import type { TreeNode } from '../types/genesis'
import { checkAvailability, fetchReleaseNotes, startScan, getScanStatus, downloadScanReport, fetchLogs, type AvailabilityResult, type ReleaseInfo, type ScanStatusResponse } from '../api/genesis'

const props = defineProps<{
  roots: TreeNode[]
  basicCharts: TreeNode[]
  basicImageComponent: Record<string, string>
  pastSelection: string
  components: string
  cniForStandard: string
  rancherVersions: string[]
  rke2Versions: string[]
  k3sVersions: string[]
  destinationRegistry?: string
}>()

const emit = defineEmits<{
  exportList: [selectedComponentIDs: string[], chartNames: string[], selectedImageRefs: string[]]
  back: []
  'update:destinationRegistry': [value: string]
}>()

// Mobile: switch between Groups / Charts / Images so each view gets full width
const mobileTab = ref<'groups' | 'charts' | 'images'>('groups')

// Flatten tree with expand state
const expanded = reactive<Record<string, boolean>>({})
const selected = reactive<Record<string, boolean>>({})

function selectAll(node: TreeNode) {
  selected[node.id] = true
  for (const c of node.children || []) selectAll(c)
}

function initSelection() {
  if (!props.roots?.length) return
  for (const r of props.roots) {
    if (r.id === 'basic') {
      selectAll(r)
      break
    }
  }
}
watch(() => props.roots, () => initSelection(), { immediate: true })

interface Row {
  depth: number
  node: TreeNode
}

const visibleRows = computed(() => {
  const out: Row[] = []
  if (!props.roots?.length) return out
  function walk(nodes: TreeNode[], depth: number) {
    for (const n of nodes) {
      out.push({ depth, node: n })
      const isExpandable =
        (n.kind === 'preset' || n.kind === 'component' || n.kind === 'chart_all' || n.kind === 'chart') &&
        n.children &&
        n.children.length > 0
      if (isExpandable && expanded[n.id]) {
        walk(n.children!, depth + 1)
      }
    }
  }
  walk(props.roots, 0)
  return out
})

function toggleExpand(id: string) {
  expanded[id] = !expanded[id]
}

function toggleSelect(row: Row) {
  const id = row.node.id
  const next = !selected[id]
  selected[id] = next
  if (row.node.children && row.node.children.length > 0) {
    const setChildren = (nodes: TreeNode[], val: boolean) => {
      for (const n of nodes) {
        selected[n.id] = val
        if (n.children?.length) setChildren(n.children, val)
      }
    }
    setChildren(row.node.children, next)
  }
}

// Build chart info: label + parent group for legend tags
interface ChartInfo { label: string; group: string }

const chartInfoMap = computed(() => {
  const m: Record<string, ChartInfo> = {}
  function walk(nodes: TreeNode[], parentGroup: string) {
    for (const n of nodes) {
      if (n.kind === 'chart') {
        m[n.id] = { label: n.label, group: parentGroup }
      }
      if (n.children) {
        const g = n.kind === 'component' ? n.label.replace(/\s*\(.*/, '') : parentGroup
        walk(n.children, g)
      }
    }
  }
  walk(props.roots, '')
  return m
})

const previewCharts = computed(() => {
  const set = new Set<string>()
  function collect(n: TreeNode) {
    if (n.kind === 'chart') set.add(n.id)
    for (const c of n.children || []) {
      if (selected[c.id]) collect(c)
    }
  }
  for (const r of visibleRows.value) {
    if (!selected[r.node.id]) continue
    if (r.node.kind === 'chart') set.add(r.node.id)
    if (r.node.children) for (const c of r.node.children) if (selected[c.id]) collect(c)
  }
  return [...set].sort()
})

const previewImages = computed(() => {
  const set = new Set<string>()
  function collect(n: TreeNode) {
    if (n.kind === 'image' && selected[n.id]) set.add(n.label)
    for (const c of n.children || []) collect(c)
  }
  for (const r of visibleRows.value) {
    if (!selected[r.node.id]) continue
    if (r.node.kind === 'image') set.add(r.node.label)
    if (r.node.children) for (const c of r.node.children) collect(c)
  }
  return [...set].sort()
})

const imageSourceGroup = computed(() => {
  const m: Record<string, string> = {}
  for (const img of previewImages.value) {
    if (props.basicImageComponent[img]) m[img] = props.basicImageComponent[img]
    else m[img] = 'addons'
  }
  return m
})

const imageChartMap = computed(() => {
  const m: Record<string, string> = {}
  function walk(nodes: TreeNode[], parentChart: string) {
    for (const n of nodes) {
      if (n.kind === 'image') {
        m[n.label] = parentChart
      }
      const chart = n.kind === 'chart' ? (chartInfoMap.value[n.id]?.label || n.label) : parentChart
      if (n.children) walk(n.children, chart)
    }
  }
  walk(props.roots, '')
  return m
})

function doExport() {
  const selectedImageRefs = previewImages.value
  const componentIDs: string[] = []
  const chartNames = previewCharts.value

  for (const r of visibleRows.value) {
    if (!selected[r.node.id]) continue
    if (r.node.id === 'basic') {
      const comps = props.components.split(',').map((c) => c.trim()).filter(Boolean)
      if (props.cniForStandard) componentIDs.push(props.cniForStandard)
      componentIDs.push('fleet')
      for (const c of comps) {
        if (c === 'k3s') componentIDs.push('k3s')
        else if (c === 'rke2') componentIDs.push('rke2')
        else if (c === 'rke') componentIDs.push('rke1')
      }
    } else if (r.node.id === 'addons') {
      // chartNames already collected from tree
    } else if (r.node.id === 'app_collection') {
      componentIDs.push('app_collection')
      componentIDs.push('app_collection_containers')
    }
  }

  emit('exportList', componentIDs, chartNames, selectedImageRefs)
}

const availResults = ref<AvailabilityResult>({})
const availLoading = ref(false)
const availChecked = ref(false)
const availError = ref('')

async function doCheckAvailability() {
  const imgs = previewImages.value
  if (imgs.length === 0) return
  availLoading.value = true
  availChecked.value = false
  availError.value = ''
  availResults.value = {}
  try {
    availResults.value = await checkAvailability(imgs)
    availChecked.value = true
  } catch (e) {
    availError.value = e instanceof Error ? e.message : String(e)
  } finally {
    availLoading.value = false
  }
}

// Scan (Trivy) – scan selected images for vulnerabilities
const scanLoading = ref(false)
const scanJobId = ref<string | null>(null)
const scanStatus = ref<ScanStatusResponse | null>(null)
const scanError = ref('')
const scanPollTimer = ref<ReturnType<typeof setInterval> | null>(null)

const scanButtonDisabled = computed(() =>
  previewImages.value.length === 0 || scanLoading.value || scanStatus.value?.status === 'running'
)
const scanButtonLabel = computed(() => {
  if (scanStatus.value?.status === 'running') return 'Ongoing scan…'
  if (scanLoading.value) return 'Scanning…'
  return 'Scan selected images'
})

async function doScan() {
  const imgs = previewImages.value
  if (imgs.length === 0) return
  scanLoading.value = true
  scanJobId.value = null
  scanStatus.value = null
  scanError.value = ''
  showScanLogs.value = true
  startScanLogsPoll()
  try {
    const { scanJobId: id } = await startScan(imgs)
    scanJobId.value = id
    const poll = async () => {
      if (!scanJobId.value) return
      try {
        const st = await getScanStatus(scanJobId.value)
        scanStatus.value = st
        if (st.status === 'running') return
        if (scanPollTimer.value) {
          clearInterval(scanPollTimer.value)
          scanPollTimer.value = null
        }
      } catch (e) {
        scanError.value = e instanceof Error ? e.message : String(e)
        if (scanPollTimer.value) {
          clearInterval(scanPollTimer.value)
          scanPollTimer.value = null
        }
      }
    }
    await poll()
    const current = scanStatus.value as ScanStatusResponse | null
    if (current?.status === 'running') {
      scanPollTimer.value = setInterval(poll, 4000)
    }
  } catch (e) {
    scanError.value = e instanceof Error ? e.message : String(e)
  } finally {
    scanLoading.value = false
  }
}

async function doDownloadScanReport() {
  if (!scanJobId.value || scanStatus.value?.status !== 'completed') return
  try {
    const blob = await downloadScanReport(scanJobId.value)
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'scan-report.csv'
    a.click()
    URL.revokeObjectURL(a.href)
  } catch (e) {
    scanError.value = e instanceof Error ? e.message : String(e)
  }
}

onUnmounted(() => {
  if (scanPollTimer.value) {
    clearInterval(scanPollTimer.value)
    scanPollTimer.value = null
  }
  if (scanLogsPollTimer.value) {
    clearInterval(scanLogsPollTimer.value)
    scanLogsPollTimer.value = null
  }
})

// Server logs during scan
const showScanLogs = ref(false)
const scanLogs = ref<string[]>([])
const scanLogsPollTimer = ref<ReturnType<typeof setInterval> | null>(null)

function startScanLogsPoll() {
  if (scanLogsPollTimer.value) return
  async function poll() {
    try {
      scanLogs.value = await fetchLogs()
    } catch {
      // ignore
    }
    if (!showScanLogs.value && !scanLoading.value && scanStatus.value?.status !== 'running') {
      if (scanLogsPollTimer.value) {
        clearInterval(scanLogsPollTimer.value)
        scanLogsPollTimer.value = null
      }
    }
  }
  poll()
  scanLogsPollTimer.value = setInterval(poll, 2000)
}

function toggleScanLogs() {
  showScanLogs.value = !showScanLogs.value
  if (showScanLogs.value) startScanLogsPoll()
  else if (!scanLoading.value && scanStatus.value?.status !== 'running' && scanLogsPollTimer.value) {
    clearInterval(scanLogsPollTimer.value)
    scanLogsPollTimer.value = null
  }
}

function chartGroupTag(group: string): string {
  if (!group) return 'A'
  const g = group.toLowerCase()
  if (g === 'rancher' || g.includes('system')) return 'R'
  if (g === 'cni') return 'C'
  if (g === 'rke2' || g === 'k3s' || g === 'rke1') return g.toUpperCase()
  if (g.includes('load') || g.includes('ingress')) return 'LB'
  return 'A'
}

function chartGroupClass(group: string): string {
  const tag = chartGroupTag(group)
  if (tag === 'R') return 'tag-Rancher'
  if (tag === 'C') return 'tag-CNI'
  if (tag === 'LB') return 'tag-LoadBalancerIngress'
  if (tag === 'RKE2' || tag === 'K3S' || tag === 'RKE1') return 'tag-RKE'
  return 'tag-addons'
}

function imgTag(group: string): string {
  if (group === 'Rancher' || group === 'Fleet') return 'R'
  if (group === 'CNI') return 'C'
  if (group.startsWith('Load')) return 'LB'
  if (group === 'K3s') return 'K3s'
  if (group === 'RKE2') return 'RKE2'
  if (group === 'RKE1') return 'RKE1'
  return 'A'
}

const availSummary = computed(() => {
  if (!availChecked.value) return null
  const r = availResults.value
  let ok = 0, notFound = 0, error = 0
  for (const key of Object.keys(r)) {
    const entry = r[key]
    if (!entry) continue
    if (entry.status === 'ok') ok++
    else if (entry.status === 'not_found') notFound++
    else error++
  }
  return { ok, notFound, error, total: ok + notFound + error }
})

// Release Notes
const releaseNotesOpen = ref(false)
const releaseNotes = ref<Record<string, ReleaseInfo>>({})
const releaseLoading = ref(false)
const releaseError = ref('')

const releaseVersions = computed(() => {
  const items: { repo: string; tag: string; label: string }[] = []
  for (const v of props.rancherVersions ?? []) {
    if (v) items.push({ repo: 'rancher/rancher', tag: v, label: `Rancher ${v}` })
  }
  for (const v of props.rke2Versions) {
    if (v && v !== 'all') items.push({ repo: 'rancher/rke2', tag: v, label: `RKE2 ${v}` })
  }
  for (const v of props.k3sVersions) {
    if (v && v !== 'all') items.push({ repo: 'rancher/k3s', tag: v, label: `K3s ${v}` })
  }
  return items
})

async function loadReleaseNotes() {
  if (releaseVersions.value.length === 0) return
  releaseLoading.value = true
  releaseError.value = ''
  releaseNotes.value = {}
  const results: Record<string, ReleaseInfo> = {}
  for (const v of releaseVersions.value) {
    try {
      results[v.label] = await fetchReleaseNotes(v.repo, v.tag)
    } catch (e) {
      results[v.label] = { tag: v.tag, name: v.label, publishedAt: '', url: '', prerelease: false, charts: [], changelog: [e instanceof Error ? e.message : String(e)] }
    }
  }
  releaseNotes.value = results
  releaseLoading.value = false
}

function toggleReleaseNotes() {
  releaseNotesOpen.value = !releaseNotesOpen.value
  if (releaseNotesOpen.value && Object.keys(releaseNotes.value).length === 0) {
    loadReleaseNotes()
  }
}
</script>

<template>
  <div class="step3">
    <div class="step3-header">
      <div class="step3-header-left">
        <h2 class="step-title">Step 3: Groups &amp; charts</h2>
        <p class="step-desc">Essentials = Rancher core, CNI, distro, LB. AddOns = monitoring, logging, etc. Toggle groups/charts/images.</p>
        <p v-if="pastSelection" class="past-selection">{{ pastSelection }}</p>
      </div>
      <div class="step3-header-actions">
        <button type="button" class="btn btn-secondary" @click="emit('back')">Back</button>
        <button
          type="button"
          class="btn btn-check"
          :disabled="availLoading || previewImages.length === 0"
          @click="doCheckAvailability"
        >
          {{ availLoading ? 'Checking…' : 'Check Availability' }}
        </button>
        <button
          type="button"
          class="btn btn-scan"
          :disabled="scanButtonDisabled"
          @click="doScan"
        >
          {{ scanButtonLabel }}
        </button>
        <button type="button" class="btn btn-primary" @click="doExport">Export image list</button>
      </div>
      <div class="destination-registry-row">
        <label class="dest-registry-label">Destination registry (optional)</label>
        <input
          type="text"
          class="dest-registry-input"
          :value="props.destinationRegistry ?? ''"
          placeholder="e.g. my-registry.example.com"
          @input="emit('update:destinationRegistry', (($event.target as HTMLInputElement)?.value ?? '').trim())"
        />
      </div>
      <div v-if="(props.destinationRegistry ?? '').length > 0" class="next-steps-box">
        <h4 class="next-steps-title">Next steps: mirror or bundle</h4>
        <p class="next-steps-desc">After exporting <code>images.txt</code>, use Hangar or <a href="https://github.com/rancher/hauler" target="_blank" rel="noopener noreferrer">Hauler</a> to mirror into your registry or create a zip bundle.</p>
        <ul class="next-steps-commands">
          <li><strong>Mirror to your registry:</strong> <code class="cmd">hangar mirror -f images.txt -d {{ props.destinationRegistry }}</code></li>
          <li><strong>Save to zip (then load on target):</strong> <code class="cmd">hangar save -f images.txt -d bundle.zip</code> then <code class="cmd">hangar load -s bundle.zip -d {{ props.destinationRegistry }}</code></li>
          <li><strong>Hauler:</strong> Use the same image list with Hauler to create a store (zip) or mirror; see <a href="https://github.com/rancher/hauler" target="_blank" rel="noopener noreferrer">Hauler docs</a>.</li>
        </ul>
      </div>
      <div v-if="scanStatus?.status === 'running'" class="scan-summary scan-ongoing">
        <span class="scan-done">Ongoing scan…</span> <span class="scan-hint">Scan selected images is disabled until the current scan finishes.</span>
      </div>
      <div v-else-if="scanStatus?.status === 'completed'" class="scan-summary">
        <span class="scan-done">Scan complete.</span>
        <template v-if="scanStatus.summary">
          <span v-if="scanStatus.summary.critical" class="scan-sev critical">{{ scanStatus.summary.critical }} critical</span>
          <span v-if="scanStatus.summary.high" class="scan-sev high">{{ scanStatus.summary.high }} high</span>
          <span v-if="scanStatus.summary.medium" class="scan-sev medium">{{ scanStatus.summary.medium }} medium</span>
          <span v-if="scanStatus.summary.low" class="scan-sev low">{{ scanStatus.summary.low }} low</span>
        </template>
        <button type="button" class="btn btn-download-report" @click="doDownloadScanReport">Download report (CSV)</button>
      </div>
      <p v-if="scanStatus?.status === 'failed' || scanError" class="error-msg">{{ scanStatus?.error || scanError }}</p>
      <div class="scan-logs-row">
        <button type="button" class="btn btn-logs-toggle" @click="toggleScanLogs">
          {{ showScanLogs ? 'Hide server logs' : 'Show server logs' }}
        </button>
        <div v-show="showScanLogs" class="scan-logs-viewer">
          <pre class="scan-logs-content">{{ scanLogs.length ? scanLogs.join('\n') : 'Waiting for server logs…' }}</pre>
        </div>
      </div>
      <div class="scan-trivy-hint">
        <strong>Scan (Trivy):</strong> Scans selected images for vulnerabilities. First run may take a few minutes (DB download). Or use CLI: <code>hangar scan -f images.txt -r scan-report.csv</code>
      </div>
      <div v-if="availChecked && availSummary" class="avail-summary">
        <span class="avail-ok">{{ availSummary.ok }}/{{ availSummary.total }} available</span>
        <span v-if="availSummary.notFound > 0" class="avail-fail">{{ availSummary.notFound }} not found</span>
        <span v-if="availSummary.error > 0" class="avail-err">{{ availSummary.error }} errors</span>
      </div>
      <p v-if="availError" class="error-msg">{{ availError }}</p>
    </div>

    <div class="tree-layout-wrapper">
      <div class="mobile-tabs" role="tablist" aria-label="Step 3 views">
        <button
          type="button"
          role="tab"
          class="mobile-tab"
          :class="{ active: mobileTab === 'groups' }"
          :aria-selected="mobileTab === 'groups'"
          @click="mobileTab = 'groups'"
        >
          <span class="mobile-tab-label">Groups</span>
        </button>
        <button
          type="button"
          role="tab"
          class="mobile-tab"
          :class="{ active: mobileTab === 'charts' }"
          :aria-selected="mobileTab === 'charts'"
          @click="mobileTab = 'charts'"
        >
          <span class="mobile-tab-label">Charts</span>
          <span class="mobile-tab-count">({{ previewCharts.length }})</span>
        </button>
        <button
          type="button"
          role="tab"
          class="mobile-tab"
          :class="{ active: mobileTab === 'images' }"
          :aria-selected="mobileTab === 'images'"
          @click="mobileTab = 'images'"
        >
          <span class="mobile-tab-label">Images</span>
          <span class="mobile-tab-count">({{ previewImages.length }})</span>
        </button>
      </div>
    <div class="tree-layout">
      <div class="col col-tree" :class="{ 'mobile-panel-active': mobileTab === 'groups' }">
        <h3 class="col-title">Groups</h3>
        <div class="tree">
          <div
            v-for="row in visibleRows"
            :key="row.node.id"
            class="tree-row"
            :style="{ paddingLeft: row.depth * 12 + 8 + 'px' }"
          >
            <span
              v-if="(row.node.kind === 'component' || row.node.kind === 'chart') && row.node.children?.length"
              class="expand"
              @click="toggleExpand(row.node.id)"
            >
              {{ expanded[row.node.id] ? '▼' : '▶' }}
            </span>
            <span v-else class="expand-placeholder"></span>
            <label class="row-label">
              <input
                type="checkbox"
                :checked="!!selected[row.node.id]"
                @change="toggleSelect(row)"
              />
              <span class="label-text">{{ row.node.label }}</span>
              <span v-if="row.node.count > 0 && row.node.kind !== 'image'" class="count">({{ row.node.count }})</span>
            </label>
          </div>
        </div>
      </div>
      <div class="col col-preview" :class="{ 'mobile-panel-active': mobileTab === 'charts' }">
        <h3 class="col-title">Charts ({{ previewCharts.length }})</h3>
        <div class="legend">
          <span class="legend-item tag-Rancher">[R] Rancher</span>
          <span class="legend-item tag-CNI">[C] CNI</span>
          <span class="legend-item tag-RKE">[RKE2] RKE2</span>
          <span class="legend-item tag-LoadBalancerIngress">[LB] Ingress</span>
          <span class="legend-item tag-addons">[A] AddOn</span>
        </div>
        <ul class="preview-list">
          <li v-for="c in previewCharts.slice(0, 80)" :key="c" class="preview-item">
            <span v-if="chartInfoMap[c]" class="img-tag" :class="chartGroupClass(chartInfoMap[c].group)">[{{ chartGroupTag(chartInfoMap[c].group) }}]</span>
            {{ chartInfoMap[c]?.label || c }}
          </li>
          <li v-if="previewCharts.length > 80" class="preview-more">… and {{ previewCharts.length - 80 }} more</li>
        </ul>
      </div>
      <div class="col col-preview" :class="{ 'mobile-panel-active': mobileTab === 'images' }">
        <h3 class="col-title">Images ({{ previewImages.length }})</h3>
        <div class="legend">
          <span class="legend-item tag-Rancher">[R] Rancher</span>
          <span class="legend-item tag-CNI">[C] CNI</span>
          <span class="legend-item tag-RKE">[RKE2] RKE2</span>
          <span class="legend-item tag-LoadBalancerIngress">[LB] Ingress</span>
          <span class="legend-item tag-addons">[A] AddOn</span>
        </div>
        <ul class="preview-list images">
          <li v-for="img in previewImages.slice(0, 150)" :key="img" class="preview-item" :title="imageChartMap[img] ? 'Chart: ' + imageChartMap[img] : ''">
            <span v-if="availChecked && availResults[img]" class="avail-dot" :class="availResults[img]?.status === 'ok' ? 'dot-ok' : 'dot-fail'" :title="availResults[img]?.detail || availResults[img]?.status">{{ availResults[img]?.status === 'ok' ? '\u2713' : '\u2717' }}</span>
            <span v-if="imageSourceGroup[img]" class="img-tag" :class="'tag-' + imageSourceGroup[img].replace(/[^a-zA-Z]/g, '')">[{{ imgTag(imageSourceGroup[img]) }}]</span>
            <span class="img-ref">{{ img }}</span>
            <span v-if="imageChartMap[img]" class="img-origin">{{ imageChartMap[img] }}</span>
          </li>
          <li v-if="previewImages.length > 150" class="preview-more">… and {{ previewImages.length - 150 }} more</li>
        </ul>
      </div>
    </div>
    </div>

    <div v-if="releaseVersions.length > 0" class="release-section">
      <button type="button" class="btn btn-release-toggle" @click="toggleReleaseNotes">
        {{ releaseNotesOpen ? '▼' : '▶' }} Release Notes &amp; Chart Versions
      </button>
      <div v-if="releaseNotesOpen" class="release-body">
        <p v-if="releaseLoading" class="loading-msg">Fetching release notes from GitHub…</p>
        <p v-if="releaseError" class="error-msg">{{ releaseError }}</p>
        <div v-for="(info, label) in releaseNotes" :key="label" class="release-card">
          <h4 class="release-card-title">
            {{ label }}
            <span v-if="info.publishedAt" class="release-date">{{ info.publishedAt.split('T')[0] }}</span>
            <a v-if="info.url" :href="info.url" target="_blank" class="release-link">GitHub ↗</a>
          </h4>

          <div v-if="info.charts && info.charts.length > 0" class="release-table-wrap">
            <h5 class="release-sub">Charts Versions</h5>
            <table class="release-table">
              <thead>
                <tr><th>Component</th><th>Version</th></tr>
              </thead>
              <tbody>
                <tr v-for="c in info.charts" :key="c.name">
                  <td>{{ c.name }}</td>
                  <td>{{ c.version }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-if="info.changelog && info.changelog.length > 0" class="release-changelog">
            <h5 class="release-sub">Changelog</h5>
            <ul class="changelog-list">
              <li v-for="(entry, i) in info.changelog" :key="i">{{ entry }}</li>
            </ul>
          </div>
          <div v-else-if="info.body" class="release-body-text">
            <h5 class="release-sub">Release notes</h5>
            <pre class="release-notes-pre">{{ info.body }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.step3 {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  flex: 1;
}
.step3-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}
.step3-header-left {
  flex: 1;
  min-width: 0;
}
.step3-header-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
}
.step-title {
  font-size: 1.25rem;
  color: var(--cyan);
  margin: 0;
}
.step-desc {
  margin: 0;
  opacity: 0.85;
  font-size: 0.9rem;
}
.past-selection {
  margin: 0;
  font-size: 0.85rem;
  color: var(--yellow);
}
.destination-registry-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 0.5rem;
  flex-wrap: wrap;
  width: 100%;
}
.dest-registry-label {
  font-size: 0.9rem;
  white-space: nowrap;
}
.dest-registry-input {
  flex: 1;
  min-width: 180px;
  max-width: 320px;
  padding: 0.35rem 0.5rem;
  font-size: 0.9rem;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg);
  color: var(--text);
}
.next-steps-box {
  margin-top: 0.75rem;
  padding: 0.75rem 1rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: color-mix(in srgb, var(--panel) 80%, var(--bg));
  width: 100%;
}
.next-steps-title {
  margin: 0 0 0.35rem 0;
  font-size: 0.95rem;
  color: var(--green);
}
.next-steps-desc {
  margin: 0 0 0.5rem 0;
  font-size: 0.85rem;
  opacity: 0.9;
}
.next-steps-desc code,
.next-steps-commands .cmd {
  background: var(--bg);
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.85em;
}
.next-steps-commands {
  margin: 0;
  padding-left: 1.25rem;
  font-size: 0.85rem;
  line-height: 1.5;
}
.next-steps-commands li {
  margin-bottom: 0.35rem;
}
.next-steps-commands a {
  color: var(--cyan);
}
.tree-layout-wrapper {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.mobile-tabs {
  display: none; /* shown only in @media (max-width: 768px) */
}
.tree-layout {
  display: grid;
  grid-template-columns: 1fr 1fr 1.2fr;
  gap: 1rem;
  flex: 1;
  min-height: 0;
}
.col {
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0.75rem;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.col-title {
  font-size: 0.95rem;
  margin: 0 0 0.5rem 0;
  color: var(--green);
}
.tree {
  flex: 1;
  overflow-y: auto;
  font-size: 0.9rem;
}
.tree-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 0;
  cursor: default;
}
.expand {
  cursor: pointer;
  width: 16px;
  color: var(--cyan);
  user-select: none;
}
.expand-placeholder {
  width: 16px;
  display: inline-block;
}
.row-label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  flex: 1;
}
.label-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.count {
  opacity: 0.8;
  font-size: 0.85em;
}
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
  font-size: 0.75rem;
  opacity: 0.85;
}
.legend-item {
  font-weight: 600;
}
.preview-list {
  flex: 1;
  overflow-y: auto;
  margin: 0;
  padding-left: 0.5rem;
  font-size: 0.8rem;
  list-style: none;
}
.preview-list.images {
  font-family: ui-monospace, monospace;
}
.preview-item {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 2px 0;
  overflow: hidden;
  white-space: nowrap;
}
.img-tag {
  margin-right: 6px;
  font-weight: 600;
  font-size: 0.85em;
}
.tag-Rancher,
.tag-Fleet {
  color: var(--cyan);
}
.tag-CNI {
  color: var(--yellow);
}
.tag-LoadBalancerIngress {
  color: #a78bfa;
}
.tag-Ks,
.tag-RKE,
.tag-RKE1 {
  color: var(--green);
}
.tag-addons {
  color: var(--green);
}
.img-ref {
  flex-shrink: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}
.img-origin {
  flex-shrink: 0;
  font-size: 0.7rem;
  opacity: 0.55;
  margin-left: auto;
  padding-left: 6px;
  white-space: nowrap;
}
.preview-more {
  opacity: 0.8;
  font-style: italic;
}
.btn {
  padding: 0.5rem 1rem;
  border-radius: 4px;
  border: 1px solid var(--border);
  cursor: pointer;
  font-size: 0.95rem;
}
.btn-primary {
  background: var(--cyan);
  color: var(--bg);
  border-color: var(--cyan);
}
.btn-secondary {
  background: var(--panel);
  color: var(--text);
}
.btn-check {
  background: var(--panel);
  color: var(--text);
  border-color: var(--border);
}
.btn-check:hover:not(:disabled) {
  border-color: var(--cyan);
}
.btn-scan {
  background: var(--panel);
  color: var(--text);
  border-color: var(--border);
}
.btn-scan:hover:not(:disabled) {
  border-color: var(--cyan);
}
.scan-summary {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.85rem;
  flex-wrap: wrap;
}
.scan-done {
  color: var(--green);
}
.scan-ongoing .scan-done {
  color: var(--cyan);
}
.scan-hint {
  font-size: 0.8rem;
  opacity: 0.85;
}
.scan-sev {
  padding: 2px 6px;
  border-radius: 3px;
}
.scan-sev.critical { background: #4a1515; color: #ffa0a0; }
.scan-sev.high { background: #4a3010; color: #ffc060; }
.scan-sev.medium { background: #2a3a2a; color: #a0d0a0; }
.scan-sev.low { background: #1a2a3a; color: #a0c0e0; }
.btn-download-report {
  font-size: 0.85rem;
  padding: 4px 10px;
  background: var(--panel);
  color: var(--cyan);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
}
.btn-download-report:hover {
  border-color: var(--cyan);
}
.scan-logs-row {
  margin-top: 0.5rem;
}
.btn-logs-toggle {
  font-size: 0.85rem;
  padding: 4px 10px;
  background: var(--panel);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
}
.btn-logs-toggle:hover {
  border-color: var(--cyan);
}
.scan-logs-viewer {
  margin-top: 0.5rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  overflow: hidden;
}
.scan-logs-content {
  margin: 0;
  padding: 0.75rem 1rem;
  font-size: 0.8rem;
  line-height: 1.4;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 280px;
  overflow: auto;
  display: block;
}
.scan-trivy-hint {
  font-size: 0.8rem;
  opacity: 0.85;
  margin-top: 0.25rem;
}
.scan-trivy-hint code {
  background: var(--bg);
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.9em;
}
.avail-summary {
  display: flex;
  gap: 1rem;
  font-size: 0.85rem;
  padding: 0.25rem 0;
}
.avail-ok {
  color: var(--green);
  font-weight: 600;
}
.avail-fail {
  color: #ef4444;
  font-weight: 600;
}
.avail-err {
  color: var(--yellow);
}
.avail-dot {
  margin-right: 4px;
  font-weight: 700;
  font-size: 0.9em;
}
.dot-ok {
  color: var(--green);
}
.dot-fail {
  color: #ef4444;
}
.error-msg {
  color: #ef4444;
  font-size: 0.85rem;
}

.release-section {
  border-top: 1px solid var(--border);
  padding-top: 0.75rem;
}
.btn-release-toggle {
  background: none;
  border: none;
  color: var(--cyan);
  cursor: pointer;
  font-size: 0.95rem;
  font-weight: 600;
  padding: 0.25rem 0;
}
.btn-release-toggle:hover {
  text-decoration: underline;
}
.release-body {
  margin-top: 0.5rem;
}
.loading-msg {
  opacity: 0.7;
  font-size: 0.9rem;
}
.release-card {
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0.75rem 1rem;
  margin-bottom: 0.75rem;
}
.release-card-title {
  font-size: 1rem;
  color: var(--green);
  margin: 0 0 0.5rem 0;
  display: flex;
  align-items: center;
  gap: 0.75rem;
}
.release-date {
  font-size: 0.8rem;
  opacity: 0.7;
  font-weight: 400;
}
.release-link {
  font-size: 0.8rem;
  color: var(--cyan);
  text-decoration: none;
  font-weight: 400;
}
.release-link:hover {
  text-decoration: underline;
}
.release-sub {
  font-size: 0.85rem;
  color: var(--yellow);
  margin: 0.5rem 0 0.25rem;
}
.release-table-wrap {
  overflow-x: auto;
}
.release-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.82rem;
}
.release-table th,
.release-table td {
  text-align: left;
  padding: 3px 12px 3px 0;
  border-bottom: 1px solid var(--border);
}
.release-table th {
  font-weight: 600;
  opacity: 0.8;
}
.changelog-list {
  padding-left: 1.25rem;
  margin: 0.25rem 0 0;
  font-size: 0.82rem;
}
.changelog-list li {
  padding: 2px 0;
}
.release-body-text {
  margin-top: 0.5rem;
}
.release-notes-pre {
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 0.82rem;
  max-height: 20em;
  overflow-y: auto;
  margin: 0.25rem 0 0;
  padding: 0.5rem;
  background: color-mix(in srgb, var(--border) 20%, transparent);
  border-radius: 4px;
}

/* Mobile & tablet */
@media (max-width: 768px) {
  .step3-header {
    flex-direction: column;
    gap: 0.75rem;
    align-items: stretch;
  }
  .step3-header-actions {
    flex-wrap: wrap;
    width: 100%;
  }
  .step3-header-actions .btn {
    flex: 1 1 auto;
    min-width: 120px;
  }
  .destination-registry-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.35rem;
  }
  .dest-registry-input {
    width: 100%;
    max-width: none;
    min-width: 0;
  }
  .next-steps-commands {
    padding-left: 1rem;
    font-size: 0.8rem;
  }
  .next-steps-commands .cmd {
    display: block;
    margin-top: 0.25rem;
    overflow-x: auto;
    white-space: pre;
    padding: 0.35rem 0.5rem;
  }
  /* Mobile: tab bar + single full-width panel */
  .mobile-tabs {
    display: flex;
    gap: 0;
    padding: 0.25rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 10px;
    margin-bottom: 0.75rem;
    flex-shrink: 0;
  }
  .mobile-tab {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
    padding: 0.6rem 0.5rem;
    font-size: 0.85rem;
    font-weight: 600;
    border: none;
    border-radius: 8px;
    background: transparent;
    color: var(--text);
    opacity: 0.75;
    cursor: pointer;
    transition: background 0.2s, opacity 0.2s;
  }
  .mobile-tab:hover {
    opacity: 1;
  }
  .mobile-tab.active {
    background: var(--panel);
    color: var(--cyan);
    opacity: 1;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  }
  .mobile-tab-count {
    font-size: 0.75rem;
    font-weight: 500;
    opacity: 0.9;
  }
  .tree-layout {
    display: flex;
    flex-direction: column;
    gap: 0;
    flex: 1;
    min-height: 0;
  }
  .tree-layout .col {
    display: none;
    flex: 1;
    min-height: 280px;
    max-height: 100%;
  }
  .tree-layout .col.mobile-panel-active {
    display: flex;
  }
  .col {
    min-height: 280px;
  }
  .tree {
    font-size: 0.85rem;
  }
  .preview-list {
    font-size: 0.75rem;
  }
  .preview-item {
    flex-wrap: wrap;
    gap: 2px;
  }
  .img-ref {
    min-width: 0;
    word-break: break-all;
    white-space: normal;
  }
  .legend {
    gap: 0.35rem;
  }
  .legend-item {
    font-size: 0.7rem;
  }
  .scan-summary {
    flex-direction: column;
    align-items: flex-start;
  }
  .avail-summary {
    flex-wrap: wrap;
    gap: 0.5rem;
  }
}

@media (max-width: 480px) {
  .step-title {
    font-size: 1.1rem;
  }
  .step-desc {
    font-size: 0.8rem;
  }
  .step3-header-actions {
    flex-direction: column;
  }
  .step3-header-actions .btn {
    width: 100%;
    min-width: 0;
  }
  .dest-registry-label {
    font-size: 0.85rem;
  }
  .next-steps-box {
    padding: 0.5rem 0.75rem;
  }
  .next-steps-title {
    font-size: 0.9rem;
  }
  .next-steps-commands li {
    margin-bottom: 0.5rem;
  }
  .next-steps-commands .cmd {
    font-size: 0.75em;
    white-space: pre-wrap;
    word-break: break-all;
  }
  .mobile-tabs {
    padding: 0.2rem;
  }
  .mobile-tab {
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.5rem 0.25rem;
    font-size: 0.8rem;
  }
  .mobile-tab-label {
    white-space: nowrap;
  }
  .mobile-tab-count {
    font-size: 0.7rem;
  }
  .tree-layout .col {
    min-height: 220px;
  }
  .col {
    padding: 0.5rem;
    min-height: 220px;
  }
  .col-title {
    font-size: 0.85rem;
  }
  .tree-row {
    padding: 3px 0;
  }
  .row-label {
    font-size: 0.85rem;
  }
  .label-text {
    white-space: normal;
    word-break: break-word;
  }
  .preview-list.images .preview-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }
  .img-ref {
    font-size: 0.7rem;
  }
  .btn,
  .btn-check,
  .btn-scan,
  .btn-primary,
  .btn-secondary {
    padding: 0.45rem 0.75rem;
    font-size: 0.88rem;
  }
  .release-card {
    padding: 0.5rem 0.75rem;
  }
  .release-card-title {
    flex-wrap: wrap;
    font-size: 0.9rem;
  }
  .release-table th,
  .release-table td {
    padding: 2px 8px 2px 0;
    font-size: 0.78rem;
  }
}
</style>
