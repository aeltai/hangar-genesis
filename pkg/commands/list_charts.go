package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cnrancher/hangar/pkg/rancher/chartimages"
	"github.com/cnrancher/hangar/pkg/rancher/kdmimages"
	"github.com/cnrancher/hangar/pkg/rancher/listgenerator"
	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

type listChartsCmd struct {
	*baseCmd
	rancherVersion string
}

func newListChartsCmd() *listChartsCmd {
	cc := &listChartsCmd{}

	cc.baseCmd = newBaseCmd(&cobra.Command{
		Use:   "list-charts",
		Short: "List all charts and their categories for a Rancher version",
		Long: `'list-charts' lists all available charts and their assigned categories
for a given Rancher version. Useful for understanding chart categorization
and planning remapping.

Example:
    hangar list-charts --rancher=v2.13.1`,
		PreRun: func(cmd *cobra.Command, args []string) {
			utils.SetupLogrus(cc.hideLogTime)
			if cc.debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cc.run()
		},
	})

	flags := cc.baseCmd.cmd.Flags()
	flags.StringVarP(&cc.rancherVersion, "rancher", "", "", "Rancher version (semver with 'v' prefix) (required)")

	return cc
}

func (cc *listChartsCmd) run() error {
	if cc.rancherVersion == "" {
		return fmt.Errorf("rancher version not specified, use '--rancher' to specify the rancher version")
	}
	if !strings.HasPrefix(cc.rancherVersion, "v") {
		cc.rancherVersion = "v" + cc.rancherVersion
	}
	if !semver.IsValid(cc.rancherVersion) {
		return fmt.Errorf("%q is not a valid semver version", cc.rancherVersion)
	}

	// Create generator to load charts (similar to generate_list.go)
	option := listgenerator.GeneratorOption{
		RancherVersion:     cc.rancherVersion,
		ChartURLs:          make(map[string]struct {
			Type   chartimages.ChartRepoType
			Branch string
		}),
		ChartsPaths:        make(map[string]chartimages.ChartRepoType),
		IncludeChartImages: true, // Include all charts
		IncludeClusterTypes: []kdmimages.ClusterType{}, // Empty means all
	}
	
	// Add default charts and KDM (same as generate-list)
	addRancherPrimeCharts(cc.rancherVersion, &option, false)
	addRancherPrimeSystemCharts(cc.rancherVersion, &option, false)
	addRancherPrimeKontainerDriverMetadata(cc.rancherVersion, &option, false)
	
	g, err := listgenerator.NewGenerator(&option)
	if err != nil {
		return fmt.Errorf("create generator: %w", err)
	}

	// Run the generator to populate images
	logrus.Info("Loading charts and images...")
	ctx, cancel := cc.baseCmd.ctxWithTimeout(0)
	defer cancel()
	if err := g.Run(ctx); err != nil {
		return fmt.Errorf("run generator: %w", err)
	}

	// Get all charts
	chartGroups := listgenerator.GroupImagesByChart(
		g.LinuxImages, g.WindowsImages)

	// Group charts by category
	chartsByCategory := make(map[string][]string)
	var allCharts []string

	for name := range chartGroups {
		allCharts = append(allCharts, name)
		cg := chartGroups[name]
		category := cg.Category
		
		// If no explicit category, infer it
		if category == "" {
			category = inferChartCategory(name)
		}
		
		if category == "" {
			category = "other"
		}
		
		chartsByCategory[category] = append(chartsByCategory[category], name)
	}

	sort.Strings(allCharts)

	// Print explicit mappings first
	fmt.Println("=== Explicitly Mapped Charts (from chartCategoryByName) ===")
	explicitMappings := map[string][]string{
		"monitoring":     {"rancher-monitoring", "rancher-monitoring-crd"},
		"logging":        {"rancher-logging", "rancher-logging-crd"},
		"backup-restore": {"rancher-backup", "rancher-backup-crd"},
		"cis":            {"rancher-cis-benchmark"},
		"fleet":          {"fleet", "fleet-crd", "fleet-agent", "fleet-controller"},
		"cluster-api":    {"rancher-cluster-api", "rancher-cluster-api-eks"},
	}
	
	for cat, charts := range explicitMappings {
		fmt.Printf("\n%s:\n", cat)
		for _, chart := range charts {
			if _, exists := chartGroups[chart]; exists {
				fmt.Printf("  ✓ %s\n", chart)
			}
		}
	}

	// Print all charts grouped by category
	fmt.Println("\n\n=== All Charts by Category (for Rancher " + cc.rancherVersion + ") ===")
	
	categoryOrder := []string{"monitoring", "logging", "backup-restore", "storage", "security", "cis", "cluster-api", "fleet", "other"}
	categoryNames := map[string]string{
		"monitoring":     "Monitoring",
		"logging":        "Logging",
		"backup-restore": "Backup & Restore",
		"storage":        "Storage",
		"security":       "Security",
		"cis":            "CIS Benchmark",
		"cluster-api":    "Cluster API",
		"fleet":          "Fleet & GitOps",
		"other":          "Other",
	}

	for _, cat := range categoryOrder {
		charts := chartsByCategory[cat]
		if len(charts) == 0 {
			continue
		}
		sort.Strings(charts)
		fmt.Printf("\n%s (%d charts):\n", categoryNames[cat], len(charts))
		for _, chart := range charts {
			cg := chartGroups[chart]
			count := 0
			if cg != nil {
				count = cg.Count()
			}
			fmt.Printf("  - %s (%d images)\n", chart, count)
		}
	}

	// Show uncategorized charts
	if uncategorized, ok := chartsByCategory[""]; ok && len(uncategorized) > 0 {
		fmt.Println("\n\nUncategorized Charts:")
		sort.Strings(uncategorized)
		for _, chart := range uncategorized {
			cg := chartGroups[chart]
			count := 0
			if cg != nil {
				count = cg.Count()
			}
			fmt.Printf("  - %s (%d images)\n", chart, count)
		}
	}

	fmt.Printf("\n\nTotal: %d charts\n", len(allCharts))
	return nil
}

// inferChartCategory infers category from chart name (same logic as in generate_list.go)
func inferChartCategory(name string) string {
	if strings.Contains(name, "monitoring") {
		return "monitoring"
	} else if strings.Contains(name, "logging") {
		return "logging"
	} else if strings.Contains(name, "backup") {
		return "backup-restore"
	} else if strings.Contains(name, "longhorn") || strings.Contains(name, "harvester") || strings.Contains(name, "storage") {
		return "storage"
	} else if strings.Contains(name, "neuvector") || strings.Contains(name, "gatekeeper") || strings.Contains(name, "security") {
		return "security"
	} else if strings.Contains(name, "cis") {
		return "cis"
	} else if strings.Contains(name, "cluster-api") {
		return "cluster-api"
	} else if strings.Contains(name, "fleet") {
		return "fleet"
	}
	return "other"
}
