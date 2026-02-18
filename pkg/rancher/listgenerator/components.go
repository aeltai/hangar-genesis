package listgenerator

import (
	"fmt"
	"strings"

	"github.com/cnrancher/hangar/pkg/utils"
)

// ComponentGroup represents a logical group of images that together implement
// a functional component such as CNI networking, DNS, monitoring, etc.
type ComponentGroup struct {
	ID          string
	Name        string
	Description string

	// LinuxImages and WindowsImages contain the image references that belong to
	// this group, keyed by image string.
	LinuxImages   map[string]bool
	WindowsImages map[string]bool
}

// Count returns the total number of distinct images (Linux + Windows) in the
// group.
func (g *ComponentGroup) Count() int {
	if g == nil {
		return 0
	}
	seen := make(map[string]bool)
	for img := range g.LinuxImages {
		seen[img] = true
	}
	for img := range g.WindowsImages {
		seen[img] = true
	}
	return len(seen)
}

// ChartComponentGroup represents the images that are pulled in by a specific
// Rancher chart/add-on.
type ChartComponentGroup struct {
	Name        string
	Category    string
	Description string

	LinuxImages   map[string]bool
	WindowsImages map[string]bool
}

// Count returns the total number of distinct images referenced by this chart
// group.
func (g *ChartComponentGroup) Count() int {
	if g == nil {
		return 0
	}
	seen := make(map[string]bool)
	for img := range g.LinuxImages {
		seen[img] = true
	}
	for img := range g.WindowsImages {
		seen[img] = true
	}
	return len(seen)
}

type componentDefinition struct {
	id          string
	name        string
	description string
	matchers    []componentMatcher
}

type componentMatcher struct {
	// Match on project/image path (without registry), e.g. "rancher/hardened-calico".
	Prefixes []string
	// Match if the image path contains any of these substrings.
	Contains []string
}

// componentDefinitions follow the modularization framework:
//   - Tier A (Core): KDM core, System Add-ons, CNI
//   - Tier B (Operational): Fleet, Monitoring, Logging, Backup, Longhorn
//   - Tier C (Compliance): CIS, NeuVector, Gatekeeper
//   - Feature Charts: source_charts (from GroupImagesBySource)
var componentDefinitions = []componentDefinition{
	// Tier A: Core Rancher Components — main server, agent, webhook, remotedialer, fleet, system-upgrade, turtles
	{
		id:          "system_addons",
		name:        "System Add-ons",
		description: "Core Rancher: rancher, rancher-agent, rancher-webhook, remotedialer-proxy, fleet, system-upgrade-controller, turtles, CoreDNS, metrics-server.",
		matchers: []componentMatcher{
			{Prefixes: []string{
				"rancher/rancher",
				"rancher/rancher-agent",
				"rancher/rancher-webhook",
				"rancher/remotedialer-proxy",
				"rancher/system-upgrade-controller",
				"rancher/turtles",
			}},
			{Contains: []string{
				"rancher-agent",
				"rancher-webhook",
				"remotedialer-proxy",
				"system-upgrade-controller",
				"coredns",
				"metrics-server",
				"k8s-dns-",
				"fleet-agent",
				"fleet-controller",
			}},
		},
	},
	// Tier A: Networking (CNI) — generic "cni" and specific CNI choices for preselect
	{
		id:          "cni",
		name:        "Networking (CNI)",
		description: "Cluster networking: Canal, Calico, Flannel, Multus, CNI plugins.",
		matchers: []componentMatcher{
			{Prefixes: []string{
				"rancher/hardened-calico",
				"rancher/hardened-flannel",
				"rancher/hardened-cni-plugins",
			}},
			{Contains: []string{
				"calico",
				"flannel",
				"cni-plugins",
				"canal",
				"multus",
			}},
		},
	},
	{
		id:          "cni_canal",
		name:        "CNI: Canal",
		description: "Canal (Calico + Flannel) CNI.",
		matchers: []componentMatcher{
			{Contains: []string{"canal"}},
		},
	},
	{
		id:          "cni_calico",
		name:        "CNI: Calico",
		description: "Calico CNI.",
		matchers: []componentMatcher{
			{Prefixes: []string{"rancher/hardened-calico"}},
			{Contains: []string{"calico"}},
		},
	},
	{
		id:          "cni_flannel",
		name:        "CNI: Flannel",
		description: "Flannel CNI.",
		matchers: []componentMatcher{
			{Prefixes: []string{"rancher/hardened-flannel"}},
			{Contains: []string{"flannel"}},
		},
	},
	{
		id:          "cni_cilium",
		name:        "CNI: Cilium",
		description: "Cilium CNI (eBPF-based networking).",
		matchers: []componentMatcher{
			{Contains: []string{"cilium"}},
		},
	},
	{
		id:          "dns",
		name:        "DNS",
		description: "CoreDNS and related DNS (when not in System Add-ons).",
		matchers: []componentMatcher{
			{Contains: []string{
				"coredns",
				"k8s-dns-",
			}},
		},
	},
	// Tier B: System Extensions
	{
		id:          "fleet",
		name:        "Fleet & GitOps",
		description: "Fleet controllers and agents for managing many clusters at scale.",
		matchers: []componentMatcher{
			{Contains: []string{
				"fleet-controller",
				"fleet-agent",
				"fleet",
			}},
		},
	},
	{
		id:          "monitoring",
		name:        "Monitoring & Observability",
		description: "AppCo stack: Alertmanager, Grafana, Thanos, kube-state-metrics, node-exporter, Redis, kube-rbac-proxy; Prometheus.",
		matchers: []componentMatcher{
			{Prefixes: []string{"rancher/appco-"}},
			{Contains: []string{
				"prometheus",
				"grafana",
				"alertmanager",
				"thanos",
				"kube-state-metrics",
				"node-exporter",
				"appco-",
			}},
		},
	},
	{
		id:          "logging",
		name:        "Logging stack",
		description: "Fluentd, Fluent Bit, and logging pipeline components.",
		matchers: []componentMatcher{
			{Contains: []string{
				"fluentd",
				"fluent-bit",
				"logging-operator",
			}},
		},
	},
	{
		id:          "backup-restore",
		name:        "Backup & Restore",
		description: "Cluster backup and restore (rancher-backup, backup-restore-operator, Velero).",
		matchers: []componentMatcher{
			{Contains: []string{
				"rancher-backup",
				"backup-restore-operator",
				"velero",
			}},
		},
	},
	{
		id:          "longhorn",
		name:        "Storage (Longhorn)",
		description: "Software-defined storage (Longhorn).",
		matchers: []componentMatcher{
			{Contains: []string{"longhorn"}},
		},
	},
	// Cloud Provider Operators (AKS, EKS, GKE, Ali, Azure Service Operator)
	{
		id:          "provisioning",
		name:        "Cloud Provider Operators",
		description: "AKS, EKS, GKE, Aliyun, Azure Service Operator.",
		matchers: []componentMatcher{
			{Contains: []string{
				"aks-operator",
				"eks-operator",
				"gke-operator",
				"ali-operator",
				"azureserviceoperator",
			}},
		},
	},
	// System management: system-agent, klipper, machine
	{
		id:          "system_agent",
		name:        "System Agent",
		description: "System agent and installer components.",
		matchers: []componentMatcher{
			{Contains: []string{
				"system-agent",
				"system-agent-installer",
			}},
		},
	},
	{
		id:          "klipper",
		name:        "Klipper (K3s)",
		description: "Klipper Helm and load balancer for K3s.",
		matchers: []componentMatcher{
			{Contains: []string{"klipper-helm", "klipper-lb"}},
		},
	},
	// Tier C: Compliance & Governance
	{
		id:          "cis",
		name:        "CIS Benchmark & Compliance",
		description: "CIS benchmark, compliance-operator, security-scan.",
		matchers: []componentMatcher{
			{Contains: []string{
				"cis-operator",
				"rancher-cis",
				"compliance-operator",
				"security-scan",
			}},
		},
	},
	{
		id:          "neuvector",
		name:        "NeuVector",
		description: "Advanced container security (NeuVector).",
		matchers: []componentMatcher{
			{Contains: []string{
				"neuvector",
			}},
		},
	},
	{
		id:          "gatekeeper",
		name:        "Gatekeeper",
		description: "OPA policy management (Gatekeeper).",
		matchers: []componentMatcher{
			{Contains: []string{
				"gatekeeper",
			}},
		},
	},
	{
		id:          "core",
		name:        "Core control-plane & addons",
		description: "Remaining K8s control-plane and addon images not in other groups.",
	},
}

// Source-based group IDs (cluster type / origin).
const (
	SourceGroupK3s                     = "source_k3s"
	SourceGroupRKE2                    = "source_rke2"
	SourceGroupRKE1                    = "source_rke1"
	SourceGroupCharts                  = "source_charts"
	SourceGroupAppCollection           = "source_app_collection"
	SourceGroupAppCollectionContainers = "app_collection_containers"
)

// Tier A: Core Infrastructure (must-have). Display order for Step 2.
var TierAOrder = []string{
	SourceGroupK3s, SourceGroupRKE2, SourceGroupRKE1,
	"system_addons", "cni",
}

// Tier B: System Extensions (operational). Display order.
var TierBOrder = []string{
	"fleet", "monitoring", "logging", "backup-restore", "longhorn",
}

// Tier C: Compliance & Governance. Display order.
var TierCOrder = []string{
	"cis", "neuvector", "gatekeeper",
}

// PriorityLevel1Preset is Minimum Viable: KDM core + System Add-ons (rancher-agent, webhook, etc.).
func PriorityLevel1Preset() []string {
	return []string{
		SourceGroupK3s, SourceGroupRKE2, SourceGroupRKE1,
		"system_addons",
	}
}

// PriorityLevel2Preset is Standard: Level 1 + CNI + Fleet.
func PriorityLevel2Preset() []string {
	return StandardPresetWithCNI("cni")
}

// StandardPresetWithCNI returns the Standard preset (Level 1 + CNI + Fleet)
// using the given CNI. Use "cni" for all CNI, "cni_canal", "cni_calico",
// "cni_flannel" for a specific CNI, or "" for no CNI in the preset.
func StandardPresetWithCNI(cni string) []string {
	out := []string{
		SourceGroupK3s, SourceGroupRKE2, SourceGroupRKE1,
		"system_addons", "fleet",
	}
	if cni != "" {
		out = append(out, cni)
	}
	return out
}

// BasicPresetWithCNI returns the Basic preset: Rancher components (system_addons) +
// selected cluster type(s) Basic (K3s/RKE2/RKE1 core) + preselected CNI.
// components should be a comma-separated string like "k3s,rke2" or "k3s" or "rke2".
func BasicPresetWithCNI(components string, cni string) []string {
	out := []string{
		"system_addons", // Rancher components
	}

	// Add selected cluster types (K3s Basic, RKE2 Basic, RKE1 Basic)
	if components != "" {
		parts := strings.Split(components, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			switch part {
			case "k3s", "1":
				out = append(out, SourceGroupK3s)
			case "rke2", "2":
				out = append(out, SourceGroupRKE2)
			case "rke", "rke1", "3":
				out = append(out, SourceGroupRKE1)
			}
		}
	} else {
		// Default: include all if nothing specified
		out = append(out, SourceGroupK3s, SourceGroupRKE2, SourceGroupRKE1)
	}

	if cni != "" {
		out = append(out, cni)
	}
	return out
}

// PriorityLevel3Preset is Full Stack: Level 2 + Monitoring + Logging + Backup.
func PriorityLevel3Preset() []string {
	return []string{
		SourceGroupK3s, SourceGroupRKE2, SourceGroupRKE1,
		"system_addons", "cni", "fleet",
		"monitoring", "logging", "backup-restore",
	}
}

// GroupImagesBySource groups images by their origin: K3s core, RKE2 core,
// RKE1 core, or Charts. Uses the source tags on each image (e.g.
// [k3s-release(rancher)], [rke2-release(rancher)], rke-system-linux,
// [path;chartName:version]). Returns the same shape as GroupImagesByComponent
// so it can be merged for display and filtering.
func GroupImagesBySource(
	linuxImages, windowsImages map[string]map[string]bool,
) map[string]*ComponentGroup {
	groups := make(map[string]*ComponentGroup)
	add := func(id, name, desc string, img string, isWindows bool) {
		g, ok := groups[id]
		if !ok {
			g = &ComponentGroup{
				ID: id, Name: name, Description: desc,
				LinuxImages:   make(map[string]bool),
				WindowsImages: make(map[string]bool),
			}
			groups[id] = g
		}
		if isWindows {
			g.WindowsImages[img] = true
		} else {
			g.LinuxImages[img] = true
		}
	}
	for img, sources := range linuxImages {
		for source := range sources {
			if strings.Contains(source, "k3s-release") || strings.Contains(source, "k3s-upgrade") {
				add(SourceGroupK3s, "K3s core", "K3s packaged components and control-plane images.", img, false)
				break
			}
		}
		for source := range sources {
			if strings.Contains(source, "rke2-release") || strings.Contains(source, "rke2-upgrade") {
				add(SourceGroupRKE2, "RKE2 core", "RKE2 packaged components and control-plane images.", img, false)
				break
			}
		}
		for source := range sources {
			if strings.Contains(source, "rke-system") {
				add(SourceGroupRKE1, "RKE1 core", "RKE (classic) system images.", img, false)
				break
			}
		}
		for source := range sources {
			if source == "[app-collection]" || (strings.Contains(source, "oci://") && strings.Contains(source, "dp.apps.rancher.io")) {
				add(SourceGroupAppCollection, "Application Collection", "Charts and container images from dp.apps.rancher.io (live-fetched).", img, false)
				break
			}
		}
		for source := range sources {
			if source == "[app-collection]" {
				add(SourceGroupAppCollectionContainers, "Application Collection (containers)", "Container-only images from dp.apps.rancher.io (no Helm chart).", img, false)
				break
			}
		}
		for source := range sources {
			if _, ok := parseChartNameFromSource(source); ok {
				add(SourceGroupCharts, "Charts / add-ons", "Rancher chart add-ons (monitoring, logging, backup, etc.).", img, false)
				break
			}
		}
	}
	for img, sources := range windowsImages {
		for source := range sources {
			if strings.Contains(source, "k3s-release") || strings.Contains(source, "k3s-upgrade") {
				add(SourceGroupK3s, "K3s core", "K3s packaged components and control-plane images.", img, true)
				break
			}
		}
		for source := range sources {
			if strings.Contains(source, "rke2-release") || strings.Contains(source, "rke2-upgrade") {
				add(SourceGroupRKE2, "RKE2 core", "RKE2 packaged components and control-plane images.", img, true)
				break
			}
		}
		for source := range sources {
			if strings.Contains(source, "rke-system") {
				add(SourceGroupRKE1, "RKE1 core", "RKE (classic) system images.", img, true)
				break
			}
		}
		for source := range sources {
			if source == "[app-collection]" || (strings.Contains(source, "oci://") && strings.Contains(source, "dp.apps.rancher.io")) {
				add(SourceGroupAppCollection, "Application Collection", "Charts and container images from dp.apps.rancher.io (live-fetched).", img, true)
				break
			}
		}
		for source := range sources {
			if source == "[app-collection]" {
				add(SourceGroupAppCollectionContainers, "Application Collection (containers)", "Container-only images from dp.apps.rancher.io (no Helm chart).", img, true)
				break
			}
		}
		for source := range sources {
			if _, ok := parseChartNameFromSource(source); ok {
				add(SourceGroupCharts, "Charts / add-ons", "Rancher chart add-ons (monitoring, logging, backup, etc.).", img, true)
				break
			}
		}
	}
	return groups
}

// GroupImagesByComponent groups the provided Linux and Windows images into
// high-level component groups based on image names. The input maps are in the
// same format as Generator.{Linux,Windows}Images.
func GroupImagesByComponent(
	linuxImages, windowsImages map[string]map[string]bool,
) map[string]*ComponentGroup {
	groups := make(map[string]*ComponentGroup)

	addImage := func(img string, isWindows bool) {
		groupIDs := classifyImageComponent(img)
		for _, id := range groupIDs {
			def := findComponentDefinition(id)
			if def == nil {
				continue
			}
			g, ok := groups[id]
			if !ok {
				g = &ComponentGroup{
					ID:            def.id,
					Name:          def.name,
					Description:   def.description,
					LinuxImages:   make(map[string]bool),
					WindowsImages: make(map[string]bool),
				}
				groups[id] = g
			}
			if isWindows {
				g.WindowsImages[img] = true
			} else {
				g.LinuxImages[img] = true
			}
		}
	}

	for img := range linuxImages {
		addImage(img, false)
	}
	for img := range windowsImages {
		addImage(img, true)
	}

	return groups
}

func findComponentDefinition(id string) *componentDefinition {
	for i := range componentDefinitions {
		if componentDefinitions[i].id == id {
			return &componentDefinitions[i]
		}
	}
	return nil
}

// classifyImageComponent returns the IDs of component groups that the given
// image belongs to. It always returns at least one ID by falling back to the
// "core" group.
func classifyImageComponent(img string) []string {
	project := utils.GetProjectName(img)
	name := utils.GetImageName(img)
	path := fmt.Sprintf("%s/%s", project, name)

	var result []string
	for _, def := range componentDefinitions {
		if def.id == "core" {
			continue
		}
		if matchesAny(path, def.matchers) {
			result = append(result, def.id)
		}
	}
	if len(result) == 0 {
		result = append(result, "core")
	}
	return result
}

func matchesAny(path string, matchers []componentMatcher) bool {
	for _, m := range matchers {
		for _, p := range m.Prefixes {
			if strings.HasPrefix(path, p) {
				return true
			}
		}
		for _, c := range m.Contains {
			if strings.Contains(path, c) {
				return true
			}
		}
	}
	return false
}

// chartCategoryByName provides optional higher-level categories for well-known
// Rancher charts. Aligned with Rancher image/chart grouping by functionality.
var chartCategoryByName = map[string]string{
	// Core Rancher / Basic (rancher-webhook, provisioning-capi, turtles, system-upgrade, remotedialer)
	"rancher-webhook":           "core",
	"rancher-provisioning-capi": "cluster-api",
	"rancher-turtles":           "cluster-api",
	"system-upgrade-controller": "core",
	"remotedialer-proxy":        "core",
	"rancher-csp-adapter":       "core",
	"ui-plugin-operator":        "core",
	"ui-plugin-operator-crd":    "core",
	"rancher-k3s-upgrader":      "core",

	// Fleet & GitOps (core: part of Basic/Rancher stack)
	"fleet":            "core",
	"fleet-crd":        "core",
	"fleet-agent":      "core",
	"fleet-controller": "core",

	// Monitoring & Observability
	"rancher-monitoring":         "monitoring",
	"rancher-monitoring-crd":     "monitoring",
	"rancher-project-monitoring": "monitoring",
	"prometheus-federator":       "monitoring",
	"rancher-alerting-drivers":   "monitoring",
	"rancher-pushprox":           "monitoring",
	"suse-observability-agent":   "monitoring",

	// Logging
	"rancher-logging":     "logging",
	"rancher-logging-crd": "logging",

	// Backup & Restore
	"rancher-backup":     "backup-restore",
	"rancher-backup-crd": "backup-restore",

	// Storage (Longhorn, Harvester, CSI)
	"longhorn":                 "storage",
	"longhorn-crd":             "storage",
	"harvester-cloud-provider": "storage",
	"harvester-csi-driver":     "storage",
	"rancher-vsphere-csi":      "storage",

	// Security (NeuVector, Gatekeeper/OPA)
	"neuvector":              "security",
	"neuvector-crd":          "security",
	"neuvector-monitor":      "security",
	"neuvector-controller":   "security",
	"neuvector-enforcer":     "security",
	"neuvector-manager":      "security",
	"rancher-gatekeeper":     "security",
	"rancher-gatekeeper-crd": "security",
	"scc-operator":           "security",

	// CIS Benchmark & Compliance
	"rancher-cis-benchmark":     "cis",
	"rancher-cis-benchmark-crd": "cis",
	"rancher-compliance":        "cis",
	"rancher-compliance-crd":    "cis",
	"compliance-operator":       "cis",

	// Provisioning (cloud provider operators, vSphere)
	"aks-operator":             "provisioning",
	"eks-operator":             "provisioning",
	"gke-operator":             "provisioning",
	"ali-operator":             "provisioning",
	"rancher-aks-operator":     "provisioning",
	"rancher-aks-operator-crd": "provisioning",
	"rancher-eks-operator":     "provisioning",
	"rancher-eks-operator-crd": "provisioning",
	"rancher-gke-operator":     "provisioning",
	"rancher-gke-operator-crd": "provisioning",
	"rancher-ali-operator":     "provisioning",
	"rancher-vsphere-cpi":      "provisioning",

	// Cluster API (CAPI) components
	"rancher-cluster-api":            "cluster-api",
	"rancher-cluster-api-eks":        "cluster-api",
	"cluster-api-controller":         "cluster-api",
	"cluster-api-aws-controller":     "cluster-api",
	"cluster-api-azure-controller":   "cluster-api",
	"cluster-api-gcp-controller":     "cluster-api",
	"cluster-api-vsphere-controller": "cluster-api",

	// Networking (Istio, SR-IOV)
	"rancher-istio": "networking",
	"sriov":         "networking",
	"sriov-crd":     "networking",

	// OS Management
	"elemental":          "os-management",
	"elemental-crd":      "os-management",
	"elemental-operator": "os-management",

	// Support & Diagnostics
	"rancher-supportability-review":     "support",
	"rancher-supportability-review-crd": "support",
}

// GroupImagesByChart groups images by the Rancher chart they originate from,
// based on the image source format used by chartimages (\"[path;chartName:version]\").
func GroupImagesByChart(
	linuxImages, windowsImages map[string]map[string]bool,
) map[string]*ChartComponentGroup {
	groups := make(map[string]*ChartComponentGroup)

	add := func(img string, sources map[string]bool, isWindows bool) {
		for source := range sources {
			chartName, ok := parseChartNameFromSource(source)
			if !ok || chartName == "" {
				continue
			}
			g, exists := groups[chartName]
			if !exists {
				g = &ChartComponentGroup{
					Name:          chartName,
					Category:      chartCategoryByName[chartName],
					LinuxImages:   make(map[string]bool),
					WindowsImages: make(map[string]bool),
				}
				groups[chartName] = g
			}
			if isWindows {
				g.WindowsImages[img] = true
			} else {
				g.LinuxImages[img] = true
			}
		}
	}

	for img, sources := range linuxImages {
		add(img, sources, false)
	}
	for img, sources := range windowsImages {
		add(img, sources, true)
	}

	return groups
}

// FilterImageSetsBySelection returns copies of linuxImages and windowsImages
// containing only images that belong to at least one of the selected component
// group IDs (including source groups: source_k3s, source_rke2, source_rke1,
// source_charts) and (if chartNames is non-empty) are either from a selected
// chart or have no chart source (e.g. KDM-only images). Empty componentGroupIDs
// or chartNames means no filter on that dimension.
func FilterImageSetsBySelection(
	linuxImages, windowsImages map[string]map[string]bool,
	componentGroupIDs, chartNames []string,
) (linuxFiltered, windowsFiltered map[string]map[string]bool) {
	componentSet := make(map[string]bool)
	for _, id := range componentGroupIDs {
		componentSet[id] = true
	}
	chartSet := make(map[string]bool)
	for _, name := range chartNames {
		chartSet[name] = true
	}
	filterComponent := len(componentSet) > 0
	filterChart := len(chartSet) > 0

	// Merge source groups (K3s/RKE2/RKE1/Charts) with functional groups (CNI, DNS, etc.)
	sourceGroups := GroupImagesBySource(linuxImages, windowsImages)
	compGroups := GroupImagesByComponent(linuxImages, windowsImages)
	merged := make(map[string]*ComponentGroup)
	for k, v := range sourceGroups {
		merged[k] = v
	}
	for k, v := range compGroups {
		merged[k] = v
	}
	// Build set of images in selected component groups.
	inSelectedComponent := make(map[string]bool)
	if filterComponent {
		for _, id := range componentGroupIDs {
			g := merged[id]
			if g == nil {
				continue
			}
			for img := range g.LinuxImages {
				inSelectedComponent[img] = true
			}
			for img := range g.WindowsImages {
				inSelectedComponent[img] = true
			}
		}
	}
	// Build set of images that have at least one source from selected charts
	// (or have no chart source).
	fromSelectedChartOrNoChart := make(map[string]bool)
	if filterChart {
		for img, sources := range linuxImages {
			hasChartSource := false
			fromSelected := false
			for source := range sources {
				chartName, ok := parseChartNameFromSource(source)
				if ok && chartName != "" {
					hasChartSource = true
					if chartSet[chartName] {
						fromSelected = true
						break
					}
				}
			}
			if !hasChartSource || fromSelected {
				fromSelectedChartOrNoChart[img] = true
			}
		}
		for img, sources := range windowsImages {
			hasChartSource := false
			fromSelected := false
			for source := range sources {
				chartName, ok := parseChartNameFromSource(source)
				if ok && chartName != "" {
					hasChartSource = true
					if chartSet[chartName] {
						fromSelected = true
						break
					}
				}
			}
			if !hasChartSource || fromSelected {
				fromSelectedChartOrNoChart[img] = true
			}
		}
	}

	include := func(img string, isWindows bool) bool {
		// If both filters are active, use OR logic: include if image matches component OR chart filter
		// This matches the TUI preview behavior where selecting components or charts shows their images
		if filterComponent && filterChart {
			// OR logic: include if in selected components OR from selected charts
			return inSelectedComponent[img] || fromSelectedChartOrNoChart[img]
		}
		// If only one filter is active, use that filter
		if filterComponent && !inSelectedComponent[img] {
			return false
		}
		if filterChart && !fromSelectedChartOrNoChart[img] {
			return false
		}
		return true
	}

	linuxFiltered = make(map[string]map[string]bool)
	windowsFiltered = make(map[string]map[string]bool)
	for img, sources := range linuxImages {
		if include(img, false) {
			linuxFiltered[img] = sources
		}
	}
	for img, sources := range windowsImages {
		if include(img, true) {
			windowsFiltered[img] = sources
		}
	}
	return linuxFiltered, windowsFiltered
}

// parseChartNameFromSource extracts the chart name from a chart source string.
// Supports: [path;chartName:version] and OCI format [oci://registry/charts/name;chartname] (no version).
// Returns the chart name and true on success, or an empty string and false if the source does not match.
func parseChartNameFromSource(source string) (string, bool) {
	// Strip leading and trailing brackets if present.
	if len(source) < 3 {
		return "", false
	}
	if source[0] == '[' && source[len(source)-1] == ']' {
		source = source[1 : len(source)-1]
	}
	semi := strings.Index(source, ";")
	if semi == -1 || semi+1 >= len(source) {
		return "", false
	}
	rest := source[semi+1:]
	colon := strings.Index(rest, ":")
	if colon == -1 {
		// OCI format: [oci://...;chartname] — no version, rest is the chart name
		chartName := strings.TrimSpace(rest)
		if chartName == "" {
			return "", false
		}
		return chartName, true
	}
	chartName := rest[:colon]
	if chartName == "" {
		return "", false
	}
	return chartName, true
}
