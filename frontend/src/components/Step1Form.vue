<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import type { Step1OptionsResponse } from '../types/genesis'
import type { RancherVersionInfo } from '../api/genesis'

const props = defineProps<{
  availableRancherVersions: RancherVersionInfo[]
  options: Step1OptionsResponse | null
  loadError: string
  optionsLoading: boolean
}>()

defineEmits<{
  generate: []
}>()

const rancherVersion = defineModel<string>('rancherVersion', { default: 'v2.13.1' })
const rancherVersions = defineModel<string[]>('rancherVersions', { default: () => ['v2.13.1'] })
const isRPMGC = defineModel<boolean>('isRPMGC', { default: false })
const includeAppCollection = defineModel<boolean>('includeAppCollection', { default: false })
const appUser = defineModel<string>('appUser', { default: '' })
const appPassword = defineModel<string>('appPassword', { default: '' })
const distros = defineModel<string[]>('distros', { default: () => ['rke2'] })
const cni = defineModel<string>('cni', { default: 'cni_calico' })
const lbK3sKlipper = defineModel<boolean>('lbK3sKlipper', { default: false })
const lbK3sTraefik = defineModel<boolean>('lbK3sTraefik', { default: false })
const lbRKE2Nginx = defineModel<boolean>('lbRKE2Nginx', { default: true })
const lbRKE2Traefik = defineModel<boolean>('lbRKE2Traefik', { default: false })
const includeRC = defineModel<boolean>('includeRC', { default: false })
const includeGitHubVersions = defineModel<boolean>('includeGitHubVersions', { default: false })
const includeWindows = defineModel<boolean>('includeWindows', { default: false })
const k3sVersions = defineModel<string[]>('k3sVersions', { default: () => ['all'] })
const rke2Versions = defineModel<string[]>('rke2Versions', { default: () => ['all'] })
const rkeVersions = defineModel<string[]>('rkeVersions', { default: () => ['all'] })

// CNI options by distro: K3s default is Flannel; RKE2 defaults to Canal and supports Calico, Cilium, Flannel; RKE1 uses Canal/Calico/Flannel
const cniOptions = computed(() => {
  const d = distros.value
  const hasK3s = d.includes('k3s')
  const hasRKE2 = d.includes('rke2')
  const hasRKE1 = d.includes('rke')
  const onlyK3s = d.length === 1 && d[0] === 'k3s'
  const onlyRKE2 = d.length === 1 && d[0] === 'rke2'
  const onlyRKE1 = d.length === 1 && d[0] === 'rke'

  if (onlyK3s) {
    return [
      { id: 'cni_flannel', label: 'Flannel', hint: 'K3s default' },
      { id: 'cni_canal', label: 'Canal', hint: 'custom' },
      { id: 'cni_calico', label: 'Calico', hint: 'custom' },
      { id: 'cni_cilium', label: 'Cilium', hint: 'custom' },
      { id: 'cni', label: 'All CNI' },
      { id: '', label: 'None' },
    ]
  }
  if (onlyRKE2) {
    return [
      { id: 'cni_canal', label: 'Canal', hint: 'RKE2 default' },
      { id: 'cni_calico', label: 'Calico' },
      { id: 'cni_cilium', label: 'Cilium' },
      { id: 'cni_flannel', label: 'Flannel' },
      { id: 'cni', label: 'All CNI' },
      { id: '', label: 'None' },
    ]
  }
  if (onlyRKE1) {
    return [
      { id: 'cni_canal', label: 'Canal', hint: 'RKE1 common' },
      { id: 'cni_calico', label: 'Calico' },
      { id: 'cni_flannel', label: 'Flannel' },
      { id: 'cni', label: 'All CNI' },
      { id: '', label: 'None' },
    ]
  }
  const base: { id: string; label: string; hint?: string }[] = []
  if (hasK3s) base.push({ id: 'cni_flannel', label: 'Flannel', hint: 'K3s default' })
  if (hasRKE2 || hasRKE1) base.push({ id: 'cni_canal', label: 'Canal', hint: hasRKE2 ? 'RKE2 default' : undefined })
  if (hasRKE2 || hasRKE1) base.push({ id: 'cni_calico', label: 'Calico' })
  if (hasRKE2) base.push({ id: 'cni_cilium', label: 'Cilium' })
  if (hasRKE1 && !base.some((x) => x.id === 'cni_flannel')) base.push({ id: 'cni_flannel', label: 'Flannel' })
  base.push({ id: 'cni', label: 'All CNI' }, { id: '', label: 'None' })
  return base
})

// When distros change, reset CNI and LB options for deselected distros
watch(
  () => [distros.value, cniOptions.value] as const,
  () => {
    const opts = cniOptions.value
    const valid = opts.some((o) => o.id === cni.value)
    const first = opts[0]
    if (!valid && first) {
      cni.value = first.id
    }
    const d = distros.value
    if (!d.includes('k3s')) {
      lbK3sKlipper.value = false
      lbK3sTraefik.value = false
    }
    if (!d.includes('rke2')) {
      lbRKE2Nginx.value = false
      lbRKE2Traefik.value = false
    }
  },
  { immediate: true }
)

function toggleVersion(arr: string[], v: string, setter: (val: string[]) => void) {
  const idx = arr.indexOf(v)
  if (idx >= 0) setter(arr.filter(x => x !== v))
  else setter([...arr, v])
}

function versionSource(distro: string, v: string): string {
  const cap = props.options?.capabilities?.[distro]
  return cap?.sources?.[v] || 'kdm'
}

function toggleDistro(d: string) {
  const i = distros.value.indexOf(d)
  if (i >= 0) {
    distros.value = distros.value.filter((x) => x !== d)
  } else {
    distros.value = [...distros.value, d]
    if (d === 'k3s') { lbK3sKlipper.value = true; lbK3sTraefik.value = true }
    if (d === 'rke2') { lbRKE2Nginx.value = true; lbRKE2Traefik.value = false }
  }
}

const rancherVersionDropdownOpen = ref(false)

function toggleRancherVersion(version: string) {
  const arr = rancherVersions.value ?? []
  const i = arr.indexOf(version)
  if (i >= 0) {
    rancherVersions.value = arr.filter((x) => x !== version)
  } else {
    rancherVersions.value = [...arr, version].sort()
  }
  rancherVersion.value = rancherVersions.value[0] ?? ''
}

function closeRancherDropdown(e: Event) {
  const target = e.target as Node
  if (rancherVersionDropdownOpen.value && !(document.querySelector('.rancher-version-dropdown')?.contains(target))) {
    rancherVersionDropdownOpen.value = false
  }
}

const rancherVersionSummary = computed(() => {
  const sel = rancherVersions.value
  if (!sel?.length) return 'Select version(s)'
  if (sel.length === 1) return sel[0]
  return `${sel.length} versions`
})

onMounted(() => {
  document.addEventListener('click', closeRancherDropdown)
})
onUnmounted(() => {
  document.removeEventListener('click', closeRancherDropdown)
})
</script>

<template>
  <div class="step1">
    <h2 class="step-title">Step 1: Source &amp; options</h2>

    <div class="field rancher-version-field">
      <label>Rancher version(s)</label>
      <p class="field-hint">Select one or more; the image list will include images for all selected versions.</p>
      <template v-if="availableRancherVersions?.length > 0">
        <div class="rancher-version-dropdown">
          <button
            type="button"
            class="rancher-version-trigger"
            :class="{ open: rancherVersionDropdownOpen }"
            @click.stop="rancherVersionDropdownOpen = !rancherVersionDropdownOpen"
          >
            <span class="trigger-text">{{ rancherVersionSummary }}</span>
            <span class="trigger-arrow">{{ rancherVersionDropdownOpen ? '▲' : '▼' }}</span>
          </button>
          <div v-show="rancherVersionDropdownOpen" class="rancher-version-panel">
            <div class="rancher-version-list">
              <label
                v-for="rv in availableRancherVersions"
                :key="rv.version"
                class="rancher-version-option"
              >
                <input
                  type="checkbox"
                  :checked="rancherVersions.includes(rv.version)"
                  @change="toggleRancherVersion(rv.version)"
                />
                <span class="option-version">{{ rv.version }}</span>
                <span v-if="rv.date" class="option-date">{{ rv.date }}</span>
              </label>
            </div>
          </div>
        </div>
      </template>
      <input
        v-else
        v-model="rancherVersion"
        type="text"
        placeholder="v2.13.1"
        class="input"
      />
      <span v-if="optionsLoading" class="loading-indicator">Loading…</span>
    </div>
    <label class="check rc-toggle">
      <input v-model="includeGitHubVersions" type="checkbox" />
      Include versions from GitHub (K3s/RKE2 release tags; shows newer than KDM)
    </label>
    <label v-if="includeGitHubVersions" class="check rc-toggle">
      <input v-model="includeRC" type="checkbox" />
      Include pre-release (RC/alpha/beta) versions from GitHub
    </label>
    <p v-if="loadError" class="error-msg">{{ loadError }}</p>

    <div class="field source-field">
      <label class="label-with-icon">
        <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/rancher.svg" alt="" class="ctx-icon" />
        Image list source
      </label>
      <div class="radio-group">
        <label class="radio">
          <input v-model="isRPMGC" type="radio" :value="false" />
          <span>Community <span class="source-detail">image lists from GitHub releases (k3s-io/k3s, rancher/rke2)</span></span>
        </label>
        <label class="radio">
          <input v-model="isRPMGC" type="radio" :value="true" />
          <span>Rancher Prime <span class="source-detail">image lists from prime.ribs.rancher.io (curated/certified)</span></span>
        </label>
      </div>
      <p class="source-note">Both sources use the same KDM (releases.rancher.com) and chart repos (rancher/charts on GitHub).</p>
    </div>

    <div class="field">
      <label class="checkbox-label">
        <input v-model="includeAppCollection" type="checkbox" />
        Include Application Collection (dp.apps.rancher.io)
      </label>
      <template v-if="includeAppCollection">
        <input v-model="appUser" type="text" placeholder="API username" class="input inline" />
        <input v-model="appPassword" type="password" placeholder="API password/token" class="input inline" />
      </template>
    </div>

    <div class="field">
      <label>Distros</label>
      <div class="check-group distros-group">
        <label class="check">
          <input type="checkbox" :checked="distros.includes('k3s')" @change="toggleDistro('k3s')" />
          <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/k3s.svg" alt="" class="ctx-icon ctx-icon-sm" />
          K3s
        </label>
        <label class="check">
          <input type="checkbox" :checked="distros.includes('rke2')" @change="toggleDistro('rke2')" />
          <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/rancher.svg" alt="" class="ctx-icon ctx-icon-sm" />
          RKE2
        </label>
        <label class="check" v-if="options?.hasRKE1">
          <input type="checkbox" :checked="distros.includes('rke')" @change="toggleDistro('rke')" />
          RKE1
        </label>
      </div>
    </div>

    <div v-if="options?.capabilities" class="field versions">
      <label class="label-with-icon">
        <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/kubernetes.svg" alt="" class="ctx-icon" />
        Kubernetes versions
      </label>
      <p v-if="!includeGitHubVersions" class="version-gh-hint">
        Only KDM-supported versions are shown. Enable <strong>Include versions from GitHub</strong> above to add newer K3s/RKE2 versions from GitHub releases.
      </p>
      <div class="version-legend">
        <span class="version-legend-item"><span class="legend-swatch swatch-kdm"></span> KDM (Rancher supported)</span>
        <span v-if="includeGitHubVersions" class="version-legend-item"><span class="legend-swatch swatch-gh"></span> GitHub release (newer)</span>
      </div>
      <div v-if="distros.includes('k3s') && options?.capabilities?.k3s" class="version-block">
        <div class="version-header">
          <span class="version-label">K3s</span>
          <label class="check version-all">
            <input type="checkbox" :checked="k3sVersions.includes('all')" @change="k3sVersions = k3sVersions.includes('all') ? [] : ['all']" />
            All
          </label>
        </div>
        <div v-if="!k3sVersions.includes('all')" class="version-chips">
          <label v-for="v in (options?.capabilities?.k3s?.versions ?? [])" :key="v" class="version-chip" :class="{ active: k3sVersions.includes(v), 'chip-github': versionSource('k3s', v) === 'github', 'chip-kdm': versionSource('k3s', v) === 'kdm' || versionSource('k3s', v) === 'both' }">
            <input type="checkbox" :checked="k3sVersions.includes(v)" @change="toggleVersion(k3sVersions, v, val => k3sVersions = val)" hidden />
            {{ v }}
            <span v-if="versionSource('k3s', v) === 'github'" class="chip-badge" title="From GitHub releases (not in KDM)">GH</span>
          </label>
        </div>
      </div>
      <div v-if="distros.includes('rke2') && options?.capabilities?.rke2" class="version-block">
        <div class="version-header">
          <span class="version-label">RKE2</span>
          <label class="check version-all">
            <input type="checkbox" :checked="rke2Versions.includes('all')" @change="rke2Versions = rke2Versions.includes('all') ? [] : ['all']" />
            All
          </label>
        </div>
        <div v-if="!rke2Versions.includes('all')" class="version-chips">
          <label v-for="v in (options?.capabilities?.rke2?.versions ?? [])" :key="v" class="version-chip" :class="{ active: rke2Versions.includes(v), 'chip-github': versionSource('rke2', v) === 'github', 'chip-kdm': versionSource('rke2', v) === 'kdm' || versionSource('rke2', v) === 'both' }">
            <input type="checkbox" :checked="rke2Versions.includes(v)" @change="toggleVersion(rke2Versions, v, val => rke2Versions = val)" hidden />
            {{ v }}
            <span v-if="versionSource('rke2', v) === 'github'" class="chip-badge" title="From GitHub releases (not in KDM)">GH</span>
          </label>
        </div>
      </div>
      <div v-if="distros.includes('rke') && options?.capabilities?.rke" class="version-block">
        <div class="version-header">
          <span class="version-label">RKE1</span>
          <label class="check version-all">
            <input type="checkbox" :checked="rkeVersions.includes('all')" @change="rkeVersions = rkeVersions.includes('all') ? [] : ['all']" />
            All
          </label>
        </div>
        <div v-if="!rkeVersions.includes('all')" class="version-chips">
          <label v-for="v in (options?.capabilities?.rke?.versions ?? [])" :key="v" class="version-chip" :class="{ active: rkeVersions.includes(v) }">
            <input type="checkbox" :checked="rkeVersions.includes(v)" @change="toggleVersion(rkeVersions, v, val => rkeVersions = val)" hidden />
            {{ v }}
          </label>
        </div>
      </div>
    </div>

    <div class="field">
      <label>Platform</label>
      <div class="radio-group">
        <label class="radio">
          <input v-model="includeWindows" type="radio" :value="false" />
          Linux only
        </label>
        <label class="radio">
          <input v-model="includeWindows" type="radio" :value="true" />
          Linux + Windows
        </label>
      </div>
    </div>

    <div v-if="distros.length > 0" class="field">
      <label>CNI</label>
      <select v-model="cni" class="input select" title="K3s default: Flannel. RKE2 default: Canal. Choose per distro docs.">
        <option v-for="o in cniOptions" :key="o.id" :value="o.id">{{ o.label }}{{ o.hint ? ' (' + o.hint + ')' : '' }}</option>
      </select>
    </div>

    <div v-if="distros.length > 0" class="field loadbalancer-field">
      <label class="label-with-icon">
        <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/nginx.svg" alt="" class="ctx-icon" />
        Load balancer / Ingress
      </label>
      <div class="check-group lb-options">
        <template v-if="distros.includes('k3s')">
          <label class="check"><input v-model="lbK3sKlipper" type="checkbox" /> K3s Klipper</label>
          <label class="check"><input v-model="lbK3sTraefik" type="checkbox" /> K3s Traefik</label>
        </template>
        <template v-if="distros.includes('rke2')">
          <label class="check"><input v-model="lbRKE2Nginx" type="checkbox" /> RKE2 NGINX</label>
          <label class="check"><input v-model="lbRKE2Traefik" type="checkbox" /> RKE2 Traefik</label>
        </template>
      </div>
    </div>

    <div class="actions">
      <button type="button" class="btn btn-primary" @click="$emit('generate')">Generate</button>
    </div>
  </div>
</template>

<style scoped>
.step1 {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.step-title {
  font-size: 1.25rem;
  color: var(--cyan);
  margin: 0 0 0.5rem 0;
}
.field {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
.field label:first-child {
  min-width: 140px;
}
.input {
  padding: 0.4rem 0.6rem;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg);
  color: var(--text);
}
.input.inline {
  margin-left: 0.5rem;
  width: 180px;
}
.select {
  min-width: 160px;
}
.select.narrow {
  min-width: 200px;
}
.radio-group,
.field-hint {
  margin: 0 0 0.5rem 0;
  font-size: 0.85rem;
  opacity: 0.88;
}
.rancher-version-field {
  flex-direction: column;
  align-items: stretch;
}
.rancher-version-dropdown {
  position: relative;
  width: 100%;
  max-width: 320px;
}
.rancher-version-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0.5rem 0.75rem;
  font-size: 0.9rem;
  font-family: inherit;
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
  text-align: left;
}
.rancher-version-trigger:hover,
.rancher-version-trigger.open {
  border-color: var(--cyan);
}
.trigger-arrow {
  font-size: 0.7rem;
  opacity: 0.8;
  margin-left: 0.5rem;
}
.rancher-version-panel {
  position: absolute;
  top: 100%;
  left: 0;
  margin-top: 4px;
  min-width: 100%;
  max-height: 280px;
  overflow-y: auto;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
  z-index: 50;
}
.rancher-version-list {
  padding: 0.35rem 0;
}
.rancher-version-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0.4rem 0.75rem;
  font-size: 0.88rem;
  cursor: pointer;
  user-select: none;
}
.rancher-version-option:hover {
  background: color-mix(in srgb, var(--cyan) 15%, transparent);
}
.rancher-version-option input {
  flex-shrink: 0;
}
.option-version {
  font-weight: 600;
}
.option-date {
  font-size: 0.8rem;
  opacity: 0.85;
}
.label-with-icon {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.ctx-icon {
  width: 20px;
  height: 20px;
  object-fit: contain;
  flex-shrink: 0;
  filter: invert(1);
}
.ctx-icon-sm {
  width: 18px;
  height: 18px;
}
[data-theme="light"] .ctx-icon {
  filter: none;
}
.distros-group .check {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.check-group {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}
.radio,
.check,
.checkbox-label {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  cursor: pointer;
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
.btn-primary:hover {
  filter: brightness(1.1);
}
.versions {
  flex-direction: column;
  align-items: flex-start;
}
.loadbalancer-field {
  flex-direction: column;
  align-items: flex-start;
}
.lb-options {
  margin-left: 0.2rem;
}
.rc-toggle {
  margin: -0.25rem 0 0 140px;
  font-size: 0.85rem;
  opacity: 0.85;
}
.loading-indicator {
  font-size: 0.85rem;
  opacity: 0.7;
  margin-left: 0.5rem;
}
.version-block {
  margin-top: 0.25rem;
}
.version-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.35rem;
}
.version-label {
  font-weight: 600;
  min-width: 40px;
}
.version-all {
  font-size: 0.85rem;
  opacity: 0.9;
}
.version-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  max-height: 120px;
  overflow-y: auto;
  padding: 2px;
}
.version-chip {
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text);
  font-size: 0.78rem;
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
  transition: background 0.15s, border-color 0.15s;
}
.version-chip:hover {
  border-color: var(--cyan);
}
.version-chip.active {
  background: var(--cyan);
  color: var(--bg);
  border-color: var(--cyan);
}
.version-chip.chip-kdm {
  border-color: var(--cyan);
}
.version-chip.chip-github {
  border-style: dashed;
}
.chip-badge {
  font-size: 0.65rem;
  font-weight: 700;
  background: var(--yellow, #eab308);
  color: #000;
  padding: 0 3px;
  border-radius: 2px;
  margin-left: 2px;
  line-height: 1.2;
}
.version-gh-hint {
  font-size: 0.85rem;
  opacity: 0.9;
  margin: 0 0 0.5rem 0;
  padding: 0.4rem 0.6rem;
  background: color-mix(in srgb, var(--cyan) 12%, transparent);
  border-radius: 4px;
  border-left: 3px solid var(--cyan);
}
.version-gh-hint strong {
  font-weight: 600;
}
.version-legend {
  display: flex;
  gap: 1rem;
  font-size: 0.78rem;
  opacity: 0.85;
  margin-bottom: 0.15rem;
}
.version-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.legend-swatch {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 2px;
}
.swatch-kdm {
  background: var(--cyan);
}
.swatch-gh {
  border: 1.5px dashed var(--yellow, #eab308);
  background: transparent;
}
.actions {
  margin-top: 0.5rem;
}
.rancher-select {
  min-width: 140px;
}
.source-field {
  flex-direction: column;
  align-items: flex-start;
}
.source-detail {
  font-size: 0.8rem;
  opacity: 0.7;
}
.source-note {
  font-size: 0.78rem;
  opacity: 0.6;
  margin: 0.25rem 0 0;
}
.data-sources {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}
.ds-title {
  font-size: 0.85rem;
  color: var(--cyan);
  margin: 0 0 0.5rem;
  font-weight: 600;
}
.ds-grid {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.ds-item {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  font-size: 0.8rem;
}
.ds-label {
  min-width: 100px;
  font-weight: 600;
  opacity: 0.85;
}
.ds-value {
  background: var(--bg);
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 0.78rem;
  word-break: break-all;
}
.error-msg {
  color: var(--red);
  font-size: 0.9rem;
  margin: 0;
}

/* Mobile & tablet */
@media (max-width: 768px) {
  .step1 {
    gap: 0.75rem;
  }
  .field {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.35rem;
  }
  .field label:first-child {
    min-width: 0;
  }
  .input.inline {
    margin-left: 0;
    width: 100%;
    max-width: 280px;
  }
  .select {
    width: 100%;
    max-width: 280px;
    min-width: 0;
  }
  .rc-toggle {
    margin-left: 0;
  }
  .rancher-version-dropdown {
    max-width: 100%;
  }
  .check-group {
    gap: 0.75rem;
  }
  .version-chips {
    max-height: 100px;
  }
  .actions .btn {
    width: 100%;
    max-width: 200px;
  }
}

@media (max-width: 480px) {
  .step-title {
    font-size: 1.1rem;
  }
  .field label:first-child {
    font-size: 0.9rem;
  }
  .input,
  .input.inline,
  .select {
    width: 100%;
    max-width: none;
    box-sizing: border-box;
  }
  .radio-group {
    margin-left: 0;
  }
  .radio span,
  .check,
  .checkbox-label {
    font-size: 0.9rem;
  }
  .source-detail {
    display: block;
    margin-top: 0.2rem;
  }
  .version-header {
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .version-label {
    min-width: 0;
    width: 100%;
  }
  .version-chips {
    max-height: 90px;
    gap: 3px;
  }
  .version-chip {
    font-size: 0.72rem;
    padding: 2px 6px;
  }
  .version-legend {
    flex-direction: column;
    gap: 0.25rem;
  }
  .ds-grid {
    gap: 0.25rem;
  }
  .ds-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.15rem;
  }
  .ds-label {
    min-width: 0;
  }
  .actions .btn {
    width: 100%;
    max-width: none;
  }
}
</style>
