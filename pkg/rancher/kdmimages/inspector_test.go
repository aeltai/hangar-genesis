package kdmimages

import (
	"testing"

	"github.com/rancher/rke/types/kdm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectClusterVersions_EmptyData(t *testing.T) {
	data := kdm.Data{}
	result, err := InspectClusterVersions("v2.8.0", "v1.25.0", true, data)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestInspectClusterVersions_InvalidRancherVersion(t *testing.T) {
	data := kdm.Data{}
	_, err := InspectClusterVersions("", "v1.25.0", true, data)
	assert.Error(t, err)
	_, err = InspectClusterVersions("2.8.0", "v1.25.0", true, data)
	assert.Error(t, err)
}

func TestInspectClusterVersions_K3SReleases(t *testing.T) {
	// Minimal KDM-like structure with one K3S release compatible with v2.8.0
	k3sData := map[string]any{
		"releases": []any{
			map[string]any{
				"version":                 "v1.25.5+k3s1",
				"minChannelServerVersion": "v2.7.0",
				"maxChannelServerVersion": "v2.9.0",
			},
		},
	}
	data := kdm.Data{K3S: k3sData}

	result, err := InspectClusterVersions("v2.8.0", "v1.25.0", true, data)
	require.NoError(t, err)
	info, ok := result[K3S]
	require.True(t, ok, "expected K3S in result")
	assert.NotEmpty(t, info.Versions)
	assert.Equal(t, K3S, info.Type)
}

func TestSortSemverList(t *testing.T) {
	versions := []string{"v1.28.1", "v1.28.3", "v1.28.2"}
	sorted := sortSemverList(versions)
	assert.Len(t, sorted, 3)
	// Descending: newest first
	assert.Equal(t, "v1.28.3", sorted[0])
	assert.Equal(t, "v1.28.2", sorted[1])
	assert.Equal(t, "v1.28.1", sorted[2])
}

func TestSortSemverList_Empty(t *testing.T) {
	assert.Nil(t, sortSemverList(nil))
	assert.Empty(t, sortSemverList([]string{}))
}
