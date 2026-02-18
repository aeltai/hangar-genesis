package kdmimages

import (
	"fmt"
	"slices"

	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/rancher/rke/types/kdm"
	"golang.org/x/mod/semver"
)

// ClusterVersionInfo describes the set of Kubernetes versions that are
// compatible with a specific cluster type for a given Rancher version.
type ClusterVersionInfo struct {
	// Type is the cluster type (K3s, RKE2, or RKE1).
	Type ClusterType
	// Versions is the list of compatible Kubernetes versions for this type.
	Versions []string
}

// InspectClusterVersions inspects the provided KDM data and returns, per
// cluster type, the Kubernetes versions that are compatible with the given
// Rancher version. It reuses the same compatibility logic as the existing
// KDM getters but performs no network I/O.
func InspectClusterVersions(
	rancherVersion, minKubeVersion string,
	removeDeprecated bool,
	data kdm.Data,
) (map[ClusterType]ClusterVersionInfo, error) {
	if rancherVersion == "" {
		return nil, fmt.Errorf("InspectClusterVersions: empty rancher version")
	}
	if !semver.IsValid(rancherVersion) {
		return nil, fmt.Errorf("InspectClusterVersions: invalid rancher version %q", rancherVersion)
	}
	if minKubeVersion != "" {
		if _, err := utils.EnsureSemverValid(minKubeVersion); err != nil {
			return nil, fmt.Errorf("InspectClusterVersions: invalid min kube version %q: %w", minKubeVersion, err)
		}
	}

	result := map[ClusterType]ClusterVersionInfo{}

	// K3s
	if data.K3S != nil {
		k3sGetter, err := newK3sRKE2Getter(&GetterOptions{
			Type:             K3S,
			RancherVersion:   rancherVersion,
			MinKubeVersion:   minKubeVersion,
			KDMData:          data,
			RemoveDeprecated: removeDeprecated,
			// TLS and IncludeVersions are not relevant for version inspection.
		})
		if err != nil {
			return nil, err
		}
		versions, err := k3sGetter.compatibleVersions()
		if err != nil {
			return nil, err
		}
		if len(versions) > 0 {
			result[K3S] = ClusterVersionInfo{
				Type:     K3S,
				Versions: sortSemverList(versions),
			}
		}
	}

	// RKE2
	if data.RKE2 != nil {
		rke2Getter, err := newK3sRKE2Getter(&GetterOptions{
			Type:             RKE2,
			RancherVersion:   rancherVersion,
			MinKubeVersion:   minKubeVersion,
			KDMData:          data,
			RemoveDeprecated: removeDeprecated,
		})
		if err != nil {
			return nil, err
		}
		versions, err := rke2Getter.compatibleVersions()
		if err != nil {
			return nil, err
		}
		if len(versions) > 0 {
			result[RKE2] = ClusterVersionInfo{
				Type:     RKE2,
				Versions: sortSemverList(versions),
			}
		}
	}

	// RKE (RKE1) – only for Rancher versions that still support it. The actual
	// compatibility is fully driven by KDM via K8sVersionInfo and related
	// metadata.
	rkeGetter, err := newRKEGetter(&GetterOptions{
		Type:           RKE,
		RancherVersion: rancherVersion,
		KDMData:        data,
	})
	if err != nil {
		// If KDM does not contain RKE data, newRKEGetter will still succeed with
		// empty maps, so reaching this error should be unexpected.
		return nil, err
	}
	if rkeGetter.versionSet == nil {
		rkeGetter.versionSet = make(map[string]bool)
	}
	if err := rkeGetter.getK8sVersionInfo(); err != nil {
		return nil, err
	}
	if len(rkeGetter.versionSet) > 0 {
		versions := make([]string, 0, len(rkeGetter.versionSet))
		for v := range rkeGetter.versionSet {
			versions = append(versions, v)
		}
		result[RKE] = ClusterVersionInfo{
			Type:     RKE,
			Versions: sortSemverList(versions),
		}
	}

	return result, nil
}

// sortSemverList sorts semantic versions in descending order (newest first),
// falling back to lexical comparison when EnsureSemverValid cannot normalize
// a value.
func sortSemverList(versions []string) []string {
	if len(versions) == 0 {
		return versions
	}
	copied := make([]string, len(versions))
	copy(copied, versions)
	slices.SortFunc(copied, func(a, b string) int {
		va, errA := utils.EnsureSemverValid(a)
		vb, errB := utils.EnsureSemverValid(b)
		if errA != nil || errB != nil {
			// Fall back to plain string comparison.
			if a == b {
				return 0
			}
			if a > b {
				return -1
			}
			return 1
		}
		// utils.SemverCompare returns >0 when va > vb.
		n, err := utils.SemverCompare(va, vb)
		if err != nil {
			if a == b {
				return 0
			}
			if a > b {
				return -1
			}
			return 1
		}
		// We want newest (highest) versions first (descending). SortFunc orders
		// by "less": negative means a < b (a before b). So we want a before b
		// when va > vb, i.e. return negative when va > vb.
		if n > 0 {
			return -1
		}
		if n < 0 {
			return 1
		}
		return 0
	})
	return copied
}
