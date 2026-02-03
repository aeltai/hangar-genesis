<script setup lang="ts">
import { computed, watch } from 'vue'
import type { Step1OptionsResponse } from '../types/genesis'

defineProps<{
  rancherVersions: string[]
  options: Step1OptionsResponse | null
  loadError: string
}>()

defineEmits<{
  loadOptions: []
  generate: []
}>()

const rancherVersion = defineModel<string>('rancherVersion', { default: 'v2.13.1' })
const isRPMGC = defineModel<boolean>('isRPMGC', { default: false })
const includeAppCollection = defineModel<boolean>('includeAppCollection', { default: false })
const appUser = defineModel<string>('appUser', { default: '' })
const appPassword = defineModel<string>('appPassword', { default: '' })
const distros = defineModel<string[]>('distros', { default: () => ['rke2'] })
const cni = defineModel<string>('cni', { default: 'cni_calico' })
const lbK3sKlipper = defineModel<boolean>('lbK3sKlipper', { default: true })
const lbK3sTraefik = defineModel<boolean>('lbK3sTraefik', { default: true })
const lbRKE2Nginx = defineModel<boolean>('lbRKE2Nginx', { default: true })
const lbRKE2Traefik = defineModel<boolean>('lbRKE2Traefik', { default: true })
const includeWindows = defineModel<boolean>('includeWindows', { default: false })
const k3sVersions = defineModel<string>('k3sVersions', { default: 'all' })
const rke2Versions = defineModel<string>('rke2Versions', { default: 'all' })
const rkeVersions = defineModel<string>('rkeVersions', { default: 'all' })

// CNI options: K3s-only => only Flannel; RKE2/RKE1 => Canal, Calico, Cilium (+ Flannel if K3s also selected)
const cniOptions = computed(() => {
  const d = distros.value
  const onlyK3s = d.length === 1 && d[0] === 'k3s'
  if (onlyK3s) {
    return [
      { id: 'cni_flannel', label: 'Flannel' },
      { id: 'cni', label: 'All CNI' },
      { id: '', label: 'None' },
    ]
  }
  const base = [
    { id: 'cni_canal', label: 'Canal' },
    { id: 'cni_calico', label: 'Calico' },
    { id: 'cni_cilium', label: 'Cilium' },
  ]
  if (d.includes('k3s')) {
    base.push({ id: 'cni_flannel', label: 'Flannel' })
  }
  base.push({ id: 'cni', label: 'All CNI' }, { id: '', label: 'None' })
  return base
})

// When distros change, if current CNI is no longer valid (e.g. Flannel with only RKE2), reset to first option
watch(
  () => [distros.value, cniOptions.value] as const,
  () => {
    const opts = cniOptions.value
    const valid = opts.some((o) => o.id === cni.value)
    const first = opts[0]
    if (!valid && first) {
      cni.value = first.id
    }
  },
  { immediate: true }
)

function toggleDistro(d: string) {
  const i = distros.value.indexOf(d)
  if (i >= 0) {
    distros.value = distros.value.filter((x) => x !== d)
  } else {
    distros.value = [...distros.value, d]
  }
}
</script>

<template>
  <div class="step1">
    <h2 class="step-title">Step 1: Source &amp; options</h2>

    <div class="field">
      <label>Rancher version</label>
      <template v-if="rancherVersions?.length > 0">
        <select v-model="rancherVersion" class="input select rancher-select">
          <option v-if="rancherVersion && !rancherVersions.includes(rancherVersion)" :value="rancherVersion">{{ rancherVersion }} (custom)</option>
          <option v-for="v in rancherVersions" :key="v" :value="v">{{ v }}</option>
        </select>
      </template>
      <input
        v-else
        v-model="rancherVersion"
        type="text"
        placeholder="v2.13.1"
        class="input"
      />
      <button type="button" class="btn btn-secondary" @click="$emit('loadOptions')">Load options</button>
    </div>
    <p class="hint">Load options fetches Rancher release channels and Kubernetes versions for each distro.</p>
    <p v-if="loadError" class="error-msg">{{ loadError }}</p>

    <div class="field">
      <label>Source</label>
      <div class="radio-group">
        <label class="radio">
          <input v-model="isRPMGC" type="radio" :value="false" />
          Community (GitHub, releases.rancher.com)
        </label>
        <label class="radio">
          <input v-model="isRPMGC" type="radio" :value="true" />
          Rancher Prime (charts.rancher.com)
        </label>
      </div>
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
      <div class="check-group">
        <label class="check"><input type="checkbox" :checked="distros.includes('k3s')" @change="toggleDistro('k3s')" /> K3s</label>
        <label class="check"><input type="checkbox" :checked="distros.includes('rke2')" @change="toggleDistro('rke2')" /> RKE2</label>
        <label class="check" v-if="options?.hasRKE1"><input type="checkbox" :checked="distros.includes('rke')" @change="toggleDistro('rke')" /> RKE1</label>
      </div>
    </div>

    <div v-if="options?.capabilities" class="field versions">
      <label>Kubernetes versions</label>
      <div v-if="distros.includes('k3s') && options?.capabilities?.k3s" class="version-row">
        <span>K3s:</span>
        <select v-model="k3sVersions" class="input select narrow">
          <option value="all">All</option>
          <option v-for="v in (options?.capabilities?.k3s?.versions ?? [])" :key="v" :value="v">{{ v }}</option>
        </select>
      </div>
      <div v-if="distros.includes('rke2') && options?.capabilities?.rke2" class="version-row">
        <span>RKE2:</span>
        <select v-model="rke2Versions" class="input select narrow">
          <option value="all">All</option>
          <option v-for="v in (options?.capabilities?.rke2?.versions ?? [])" :key="v" :value="v">{{ v }}</option>
        </select>
      </div>
      <div v-if="distros.includes('rke') && options?.capabilities?.rke" class="version-row">
        <span>RKE1:</span>
        <select v-model="rkeVersions" class="input select narrow">
          <option value="all">All</option>
          <option v-for="v in (options?.capabilities?.rke?.versions ?? [])" :key="v" :value="v">{{ v }}</option>
        </select>
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
      <select v-model="cni" class="input select">
        <option v-for="o in cniOptions" :key="o.id" :value="o.id">{{ o.label }}</option>
      </select>
    </div>

    <div v-if="distros.length > 0" class="field loadbalancer-field">
      <label>Load balancer / Ingress</label>
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
.btn-secondary {
  background: var(--panel);
  color: var(--text);
  margin-left: 0.5rem;
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
.version-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 0.25rem;
}
.actions {
  margin-top: 0.5rem;
}
.rancher-select {
  min-width: 140px;
}
.hint {
  margin: 0 0 0 140px;
  font-size: 0.85rem;
  opacity: 0.85;
}
.error-msg {
  color: var(--red);
  font-size: 0.9rem;
  margin: 0;
}
</style>
