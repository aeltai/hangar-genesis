# Current Chart Categorization Mapping (v2.13.1)

## 1. Explicit Mappings (`pkg/rancher/listgenerator/components.go`)

```go
var chartCategoryByName = map[string]string{
    "rancher-monitoring":      "monitoring",
    "rancher-monitoring-crd":  "monitoring",
    "rancher-logging":         "logging",
    "rancher-logging-crd":     "logging",
    "rancher-backup":          "backup-restore",
    "rancher-backup-crd":      "backup-restore",
    "rancher-cis-benchmark":   "cis",
    "fleet":                   "fleet",
    "fleet-crd":               "fleet",
    "fleet-agent":             "fleet",
    "fleet-controller":        "fleet",
    "rancher-cluster-api":     "cluster-api",
    "rancher-cluster-api-eks": "cluster-api",
}
```

**Total: 12 explicit mappings**

## 2. Inference Logic (`pkg/commands/generate_list.go`, lines 1086-1104)

If a chart name is NOT in the explicit mapping above, it's categorized by string matching:

| Pattern | Category | Notes |
|---------|----------|-------|
| Contains `"monitoring"` | `monitoring` | Monitoring stack |
| Contains `"logging"` | `logging` | Logging stack |
| Contains `"backup"` | `backup-restore` | Backup & restore |
| Contains `"longhorn"` OR `"harvester"` OR `"storage"` | `storage` | Storage components |
| Contains `"neuvector"` OR `"gatekeeper"` OR `"security"` | `security` | Security & policy |
| Contains `"cis"` | `cis` | CIS Benchmark |
| Contains `"cluster-api"` | `cluster-api` | Cluster API |
| Contains `"fleet"` | `fleet` | Fleet & GitOps (also treated as Basic) |
| Otherwise | `other` | Uncategorized |

## 3. Category Display Order (Step 2 TUI)

Categories are displayed in this order:
1. `monitoring` → "Monitoring"
2. `logging` → "Logging"
3. `backup-restore` → "Backup & Restore"
4. `storage` → "Storage"
5. `security` → "Security"
6. `cis` → "CIS Benchmark"
7. `cluster-api` → "Cluster API"
8. `other` → "Other"

## 4. Current Limitations

- **No explicit provisioning category**: EKS, GKE, AKS provisioning images are currently inferred as `cluster-api` or `other`
- **No OS management category**: Elemental operator images fall into `other`
- **Limited hardened image recognition**: Hardened images (hardened-kubernetes, hardened-calico, etc.) are not explicitly categorized
- **No appco-* pattern**: App ecosystem images (appco-redis, appco-thanos) are not explicitly handled
- **No distro-specific patterns**: RKE2/K3s core images are handled by component groups, not chart categories

## 5. Files to Update

1. **`pkg/rancher/listgenerator/components.go`** (line 524): Add explicit chart mappings
2. **`pkg/commands/generate_list.go`** (line 1086): Update inference logic with new patterns
3. **`pkg/commands/generate_list.go`** (line 1093): Update category order and names
