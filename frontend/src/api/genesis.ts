import type {
  Step1OptionsResponse,
  GenerateRequest,
  GenerateResponse,
  ExportRequest,
} from '../types/genesis'

const API_BASE = '/api'

export async function fetchRancherVersions(): Promise<string[]> {
  const r = await fetch(`${API_BASE}/rancher-versions`)
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  const data = await r.json() as { versions: string[] }
  return data.versions ?? []
}

export async function fetchStep1Options(rancherVersion: string): Promise<Step1OptionsResponse> {
  const v = encodeURIComponent(rancherVersion)
  const r = await fetch(`${API_BASE}/step1-options?rancher=${v}`)
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

export async function generate(req: GenerateRequest): Promise<GenerateResponse> {
  const r = await fetch(`${API_BASE}/generate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

export async function fetchLogs(): Promise<string[]> {
  const r = await fetch(`${API_BASE}/logs`)
  if (!r.ok) return []
  const data = (await r.json()) as { lines?: string[] }
  return data.lines ?? []
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
