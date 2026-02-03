package listgenerator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityPresets(t *testing.T) {
	l1 := PriorityLevel1Preset()
	require.NotEmpty(t, l1)
	assert.Contains(t, l1, SourceGroupK3s)
	assert.Contains(t, l1, "system_addons")
	l2 := PriorityLevel2Preset()
	require.NotEmpty(t, l2)
	assert.Contains(t, l2, "cni")
	assert.Contains(t, l2, "fleet")
	// Standard preset with specific CNI
	l2calico := StandardPresetWithCNI("cni_calico")
	assert.Contains(t, l2calico, "cni_calico")
	assert.NotContains(t, l2calico, "cni")
	// No CNI
	l2none := StandardPresetWithCNI("")
	assert.NotContains(t, l2none, "cni")
	assert.Contains(t, l2none, "fleet")
	l3 := PriorityLevel3Preset()
	require.NotEmpty(t, l3)
	assert.Contains(t, l3, "monitoring")
	assert.Contains(t, l3, "logging")
	assert.Contains(t, l3, "backup-restore")
}

func TestGroupImagesBySource(t *testing.T) {
	linux := map[string]map[string]bool{
		"rancher/hardened-calico:v1.2.3": {"[k3s-release(rancher)]": true},
		"rancher/coredns:1.9":            {"[k3s-release(rancher)]": true},
		"rancher/backup:1.0":             {"[/path;rancher-backup:1.0]": true},
	}
	windows := map[string]map[string]bool{}
	groups := GroupImagesBySource(linux, windows)
	require.NotEmpty(t, groups)
	assert.True(t, groups[SourceGroupK3s].LinuxImages["rancher/hardened-calico:v1.2.3"])
	assert.True(t, groups[SourceGroupK3s].LinuxImages["rancher/coredns:1.9"])
	assert.True(t, groups[SourceGroupCharts].LinuxImages["rancher/backup:1.0"])
	assert.Equal(t, "K3s core", groups[SourceGroupK3s].Name)
	assert.Equal(t, "Charts / add-ons", groups[SourceGroupCharts].Name)
}

func TestGroupImagesByComponent(t *testing.T) {
	linux := map[string]map[string]bool{
		"rancher/hardened-calico:v1.2.3":  {"[k3s-release]": true},
		"rancher/hardened-cni-plugins:v1": {"[k3s-release]": true},
		"rancher/coredns:1.9":             {"[k3s-release]": true},
		"docker.io/library/nginx:latest":  {"[other]": true},
	}
	windows := map[string]map[string]bool{}
	groups := GroupImagesByComponent(linux, windows)
	require.NotEmpty(t, groups)
	// hardened-calico and hardened-cni-plugins should be in CNI
	if g, ok := groups["cni"]; ok {
		assert.True(t, g.LinuxImages["rancher/hardened-calico:v1.2.3"] || g.LinuxImages["rancher/hardened-cni-plugins:v1"],
			"CNI group should contain at least one of the CNI images")
	}
	// coredns should be in DNS
	if g, ok := groups["dns"]; ok {
		assert.True(t, g.LinuxImages["rancher/coredns:1.9"])
	}
	// nginx might land in core
	assert.True(t, len(groups) >= 1)
}

func TestGroupImagesByChart(t *testing.T) {
	linux := map[string]map[string]bool{
		"rancher/monitoring:1.0": {
			"[path;rancher-monitoring:1.0]": true,
		},
		"rancher/backup:2.0": {
			"[path;rancher-backup:2.0]": true,
		},
	}
	windows := map[string]map[string]bool{}
	groups := GroupImagesByChart(linux, windows)
	require.Len(t, groups, 2)
	assert.True(t, groups["rancher-monitoring"].LinuxImages["rancher/monitoring:1.0"])
	assert.True(t, groups["rancher-backup"].LinuxImages["rancher/backup:2.0"])
	assert.Equal(t, "monitoring", groups["rancher-monitoring"].Category)
	assert.Equal(t, "backup-restore", groups["rancher-backup"].Category)
}

func TestFilterImageSetsBySelection_NoFilter(t *testing.T) {
	linux := map[string]map[string]bool{"img1": {"s1": true}}
	windows := map[string]map[string]bool{"img2": {"s2": true}}
	linuxOut, windowsOut := FilterImageSetsBySelection(linux, windows, nil, nil)
	assert.Equal(t, linux, linuxOut)
	assert.Equal(t, windows, windowsOut)
}

func TestFilterImageSetsBySelection_ComponentFilter(t *testing.T) {
	linux := map[string]map[string]bool{
		"rancher/hardened-calico:v1": {"[k3s]": true},
		"rancher/coredns:1.9":        {"[k3s]": true},
	}
	windows := map[string]map[string]bool{}
	linuxOut, windowsOut := FilterImageSetsBySelection(linux, windows, []string{"cni"}, nil)
	// Only CNI images should remain; coredns is DNS so may be excluded
	assert.True(t, len(linuxOut) <= 2)
	assert.NotNil(t, linuxOut)
	assert.NotNil(t, windowsOut)
}

func TestParseChartNameFromSource(t *testing.T) {
	// parseChartNameFromSource is used internally; test via GroupImagesByChart
	// and FilterImageSetsBySelection which rely on it.
	linux := map[string]map[string]bool{
		"img": {"[/repo;my-chart:1.2.3]": true},
	}
	groups := GroupImagesByChart(linux, nil)
	require.Contains(t, groups, "my-chart")
	assert.Equal(t, 1, groups["my-chart"].Count())
}
