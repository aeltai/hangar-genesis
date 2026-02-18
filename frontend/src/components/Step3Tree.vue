<script setup lang="ts">
import { reactive, computed, watch } from 'vue'
import type { TreeNode } from '../types/genesis'

const props = defineProps<{
  roots: TreeNode[]
  basicCharts: TreeNode[]
  basicImageComponent: Record<string, string>
  pastSelection: string
  components: string
  cniForStandard: string
}>()

const emit = defineEmits<{
  exportList: [selectedComponentIDs: string[], chartNames: string[], selectedImageRefs: string[]]
  back: []
}>()

// Flatten tree with expand state
const expanded = reactive<Record<string, boolean>>({})
const selected = reactive<Record<string, boolean>>({})

// Pre-select "basic" and all its direct children by default
function initSelection() {
  if (!props.roots?.length) return
  for (const r of props.roots) {
    if (r.id === 'basic') {
      selected[r.id] = true
      for (const c of r.children || []) {
        selected[c.id] = true
        if (c.children) {
          for (const gc of c.children) {
            selected[gc.id] = true
          }
        }
      }
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

// Preview: charts and images from selected nodes (simplified mirror of TUI getChartsForSelectedGroup)
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
        <button type="button" class="btn btn-primary" @click="doExport">Export image list</button>
      </div>
    </div>

    <div class="tree-layout">
      <div class="col col-tree">
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
      <div class="col col-preview">
        <h3 class="col-title">Charts</h3>
        <ul class="preview-list">
          <li v-for="c in previewCharts.slice(0, 80)" :key="c" class="preview-item">• {{ c }}</li>
          <li v-if="previewCharts.length > 80" class="preview-more">… and {{ previewCharts.length - 80 }} more</li>
        </ul>
      </div>
      <div class="col col-preview">
        <h3 class="col-title">Images ({{ previewImages.length }})</h3>
        <ul class="preview-list images">
          <li v-for="img in previewImages.slice(0, 150)" :key="img" class="preview-item">
            <span v-if="imageSourceGroup[img]" class="img-tag" :class="'tag-' + imageSourceGroup[img]">[{{ imageSourceGroup[img] === 'Rancher' || imageSourceGroup[img] === 'Fleet' ? 'R' : imageSourceGroup[img] === 'CNI' ? 'C' : imageSourceGroup[img] === 'LB' ? 'LB' : 'D' }}]</span>
            {{ img }}
          </li>
          <li v-if="previewImages.length > 150" class="preview-more">… and {{ previewImages.length - 150 }} more</li>
        </ul>
      </div>
    </div>
  </div>
</template>

<style scoped>
.step3 {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
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
.tree-layout {
  display: grid;
  grid-template-columns: 1fr 1fr 1.2fr;
  gap: 1rem;
  min-height: 400px;
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
.preview-list {
  flex: 1;
  overflow-y: auto;
  margin: 0;
  padding-left: 1.2rem;
  font-size: 0.8rem;
  list-style: none;
}
.preview-list.images {
  font-family: ui-monospace, monospace;
}
.preview-item {
  padding: 2px 0;
  overflow: hidden;
  text-overflow: ellipsis;
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
.tag-LB {
  color: #a78bfa;
}
.tag-addons {
  color: var(--green);
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
</style>
