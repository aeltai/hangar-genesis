import type {
  Step1OptionsResponse,
  GenerateRequest,
  GenerateResponse,
  ExportRequest,
} from '../types/genesis'

const API_BASE = '/api'

export interface RancherVersionInfo {
  version: string
  date: string
}

export async function fetchRancherVersions(includeRC = false): Promise<RancherVersionInfo[]> {
  const rc = includeRC ? '?includeRC=true' : ''
  const r = await fetch(`${API_BASE}/rancher-versions${rc}`)
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  const data = await r.json() as { versions: RancherVersionInfo[] | string[] }
  if (!data.versions?.length) return []
  if (typeof data.versions[0] === 'string') {
    return (data.versions as string[]).map(v => ({ version: v, date: '' }))
  }
  return data.versions as RancherVersionInfo[]
}

export async function fetchStep1Options(
  rancherVersion: string,
  includeRC = false,
  includeGitHubVersions = false
): Promise<Step1OptionsResponse> {
  const v = encodeURIComponent(rancherVersion)
  const rc = includeRC ? '&includeRC=true' : ''
  const gh = includeGitHubVersions ? '&includeGitHubVersions=true' : ''
  const r = await fetch(`${API_BASE}/step1-options?rancher=${v}${rc}${gh}`)
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

export async function generate(req: GenerateRequest): Promise<GenerateResponse> {
  const distros = req.distros
  const payload: Record<string, unknown> = {
    ...req,
    k3sVersions: distros.includes('k3s') ? req.k3sVersions.join(',') : '',
    rke2Versions: distros.includes('rke2') ? req.rke2Versions.join(',') : '',
    rkeVersions: distros.includes('rke') ? req.rkeVersions.join(',') : '',
  }
  if (req.rancherVersions?.length) {
    payload.rancherVersions = req.rancherVersions
    payload.rancherVersion = req.rancherVersions[0] || req.rancherVersion
  }
  const r = await fetch(`${API_BASE}/generate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

/** Fetch step1 options for multiple Rancher versions and merge capabilities (union of K3s/RKE2/RKE versions). */
export async function fetchStep1OptionsMerged(
  rancherVersions: string[],
  includeRC: boolean,
  includeGitHubVersions: boolean
): Promise<Step1OptionsResponse> {
  if (rancherVersions.length === 0) {
    return { hasRKE1: false, capabilities: {}, details: { kdmUrl: '', imageListSource: '' } }
  }
  if (rancherVersions.length === 1) {
    const v = rancherVersions[0]
    return v ? fetchStep1Options(v, includeRC, includeGitHubVersions) : Promise.resolve({ hasRKE1: false, capabilities: {}, details: { kdmUrl: '', imageListSource: '' } })
  }
  const results = await Promise.all(
    rancherVersions.map((v) => fetchStep1Options(v, includeRC, includeGitHubVersions))
  )
  const first = results[0]
  const merged: Step1OptionsResponse = {
    hasRKE1: results.some((r) => r.hasRKE1),
    capabilities: {},
    details: first ? first.details : { kdmUrl: '', imageListSource: '' },
  }
  const distros = ['k3s', 'rke2', 'rke'] as const
  for (const d of distros) {
    const allVersions = new Set<string>()
    const sources: Record<string, string> = {}
    for (const r of results) {
      const cap = r.capabilities?.[d]
      if (!cap) continue
      for (const v of cap.versions) {
        allVersions.add(v)
        sources[v] = cap.sources?.[v] ?? 'kdm'
      }
    }
    if (allVersions.size) {
      merged.capabilities[d] = {
        versions: [...allVersions].sort(),
        sources,
      }
    }
  }
  return merged
}

export async function fetchLogs(): Promise<string[]> {
  const r = await fetch(`${API_BASE}/logs`)
  if (!r.ok) return []
  const data = (await r.json()) as { lines?: string[] }
  return data.lines ?? []
}

export type AvailabilityResult = Record<string, { status: string; detail: string }>

export async function checkAvailability(images: string[]): Promise<AvailabilityResult> {
  const r = await fetch(`${API_BASE}/check-availability`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ images }),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  const data = await r.json() as { results: AvailabilityResult }
  return data.results
}

export interface ReleaseInfo {
  tag: string
  name: string
  publishedAt: string
  url: string
  prerelease: boolean
  charts: { name: string; version: string }[]
  changelog: string[]
  body?: string
}

export async function fetchReleaseNotes(repo: string, tag: string): Promise<ReleaseInfo> {
  const r = await fetch(`${API_BASE}/release-notes?repo=${encodeURIComponent(repo)}&tag=${encodeURIComponent(tag)}`)
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

export async function exportImageList(req: ExportRequest): Promise<Blob> {
  const r = await fetch(`${API_BASE}/export`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.blob()
}

export interface ScanStatusResponse {
  status: 'running' | 'completed' | 'failed'
  error?: string
  summary?: { critical: number; high: number; medium: number; low: number }
}

export async function startScan(images: string[]): Promise<{ scanJobId: string }> {
  const r = await fetch(`${API_BASE}/scan`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ images }),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

export async function getScanStatus(scanJobId: string): Promise<ScanStatusResponse> {
  const r = await fetch(`${API_BASE}/scan/status/${encodeURIComponent(scanJobId)}`)
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

export async function downloadScanReport(scanJobId: string): Promise<Blob> {
  const r = await fetch(`${API_BASE}/scan/report/${encodeURIComponent(scanJobId)}`)
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.blob()
}

/** Ask the backend to generate a Docker/containers auth file for the given registry credentials; triggers download of auth.json. */
export async function downloadRegistryAuthFile(
  destinationRegistry: string,
  destinationRegistryUser: string,
  destinationRegistryPassword: string
): Promise<void> {
  const r = await fetch(`${API_BASE}/genesis/registry-auth`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      destinationRegistry: destinationRegistry.trim(),
      destinationRegistryUser: destinationRegistryUser.trim(),
      destinationRegistryPassword: destinationRegistryPassword,
    }),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  const blob = await r.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'auth.json'
  a.click()
  URL.revokeObjectURL(url)
}
