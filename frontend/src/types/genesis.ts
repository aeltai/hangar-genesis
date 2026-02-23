// API types matching the Go genesis serve API

export interface ClusterVersionInfo {
  versions: string[]
  sources?: Record<string, string> // version -> "kdm" | "github" | "both"
}

export interface Step1Details {
  kdmUrl: string
  imageListSource: string
}

export interface Step1OptionsResponse {
  hasRKE1: boolean
  capabilities: Record<string, ClusterVersionInfo>
  details: Step1Details
}

export interface TreeNode {
  id: string
  label: string
  kind: string
  count: number
  children?: TreeNode[]
}

export interface GenerateRequest {
  rancherVersion: string
  rancherVersions?: string[]
  isRPMGC: boolean
  includeAppCollectionCharts: boolean
  appCollectionAPIUser: string
  appCollectionAPIPassword: string
  distros: string[]
  cni: string
  loadBalancer: boolean
  lbK3sKlipper: boolean
  lbK3sTraefik: boolean
  lbRKE2Nginx: boolean
  lbRKE2Traefik: boolean
  includeWindows: boolean
  k3sVersions: string[]
  rke2Versions: string[]
  rkeVersions: string[]
  /** Optional destination registry for mirror/save/load and Hauler; used in Next steps commands. */
  destinationRegistry?: string
  /** Optional registry username for login command hint. */
  destinationRegistryUser?: string
  /** Optional registry password/token (never echoed in UI). */
  destinationRegistryPassword?: string
}

export interface GenerateResponse {
  jobId: string
  roots: TreeNode[]
  basicCharts: TreeNode[]
  basicImageComponent: Record<string, string>
  pastSelection: string
}

export interface ExportRequest {
  jobId: string
  selectedComponentIDs: string[]
  chartNames: string[]
  selectedImageRefs: string[]
}

/** Product install instructions (Helm repo, install cmd, docs). */
export interface HelmProduct {
  description?: string
  notes?: string
  helmRepoName: string
  helmRepoUrl: string
  helmInstallCmd: string
  cliReleasesUrl?: string
  docsUrl?: string
}
