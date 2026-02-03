// API types matching the Go genesis serve API

export interface ClusterVersionInfo {
  versions: string[]
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
  k3sVersions: string
  rke2Versions: string
  rkeVersions: string
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
