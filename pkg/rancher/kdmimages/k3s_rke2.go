package kdmimages

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

const (
	// The "images-all" file is only provided for RKE2 amd64 images. This may be subject to change.
	RKE2ImageLinux   = "https://github.com/rancher/rke2/releases/download/%s/rke2-images-all.linux-amd64.txt"
	RKE2ImageWindows = "https://github.com/rancher/rke2/releases/download/%s/rke2-images.windows-amd64.txt"
	K3SImageURL      = "https://github.com/k3s-io/k3s/releases/download/%s/k3s-images.txt"

	// CN mirror URL (some versions were not mirrored to the CN mirror, so use the global GitHub instead)
	RKE2ImageLinuxCN   = "https://rancher-mirror.rancher.cn/rke2/%s/rke2-images-all.linux-amd64.txt"
	RKE2ImageWindowsCN = "https://rancher-mirror.rancher.cn/rke2/%s/rke2-images.windows-amd64.txt"
	K3SImageURLCN      = "https://rancher-mirror.rancher.cn/k3s/%s/k3s-images.txt"
)

// k3sRKE2Getter is the object to get RKE2 and K3s release & upgrade images
type k3sRKE2Getter struct {
	source             ClusterType
	rancherVersion     string
	data               map[string]any
	insecureSkipVerify bool
	removeDeprecated   bool
	includeVersions    map[string]bool // when non-empty, only fetch these k8s versions
	imageListBaseURL   string         // when set, use this base for image list URLs (e.g. Prime)

	linuxImageSet   map[string]map[string]bool
	windowsImageSet map[string]map[string]bool
	versionSet      map[string]bool
}

func newK3sRKE2Getter(o *GetterOptions) (*k3sRKE2Getter, error) {
	var data map[string]any
	switch o.Type {
	case K3S:
		data = o.KDMData.K3S
	case RKE2:
		data = o.KDMData.RKE2
	default:
		return nil, fmt.Errorf("invalid cluster type: %v", o.Type)
	}
	if _, err := utils.EnsureSemverValid(o.RancherVersion); err != nil {
		return nil, err
	}
	if _, err := utils.EnsureSemverValid(o.MinKubeVersion); err != nil {
		return nil, err
	}

	includeVersions := make(map[string]bool)
	for _, v := range o.IncludeVersions {
		if v != "" {
			includeVersions[v] = true
		}
	}
	return &k3sRKE2Getter{
		source:             o.Type,
		rancherVersion:     o.RancherVersion,
		data:               data,
		insecureSkipVerify: o.InsecureSkipTLS,
		removeDeprecated:   o.RemoveDeprecated,
		includeVersions:    includeVersions,
		imageListBaseURL:   strings.TrimSuffix(o.ImageListBaseURL, "/"),

		linuxImageSet:   make(map[string]map[string]bool),
		windowsImageSet: make(map[string]map[string]bool),
		versionSet:      make(map[string]bool),
	}, nil
}

// compatibleVersions returns the list of Kubernetes versions compatible with
// the getter's Rancher version from KDM data, without performing network I/O.
// Used by the inspector for version selection in the TUI.
func (g *k3sRKE2Getter) compatibleVersions() ([]string, error) {
	releases, ok := g.data["releases"].([]any)
	if !ok {
		return nil, fmt.Errorf("UpgradeGetter: failed to get 'releases' from data")
	}
	var compatibleVersions = []string{}
	for _, release := range releases {
		releaseMap, ok := release.(map[string]any)
		if !ok {
			continue
		}

		kubeVersion, ok := releaseMap["version"].(string)
		if !ok || kubeVersion == "" {
			continue
		}

		if g.rancherVersion == "dev" {
			compatibleVersions = append(compatibleVersions, kubeVersion)
			continue
		}
		maxVersion, ok := releaseMap["maxChannelServerVersion"].(string)
		if !ok || !semver.IsValid(maxVersion) {
			continue
		}
		minVersion, ok := releaseMap["minChannelServerVersion"].(string)
		if !ok || !semver.IsValid(minVersion) {
			continue
		}
		if semver.Compare(g.rancherVersion, minVersion) < 0 {
			continue
		}
		if semver.Compare(g.rancherVersion, maxVersion) > 0 {
			continue
		}
		compatibleVersions = append(compatibleVersions, kubeVersion)
	}
	if g.removeDeprecated {
		compatibleVersions = filterDeprecatedVersions(compatibleVersions)
	}
	return compatibleVersions, nil
}

func (g *k3sRKE2Getter) Get(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	logrus.Infof("Fetching [%v] images.", g.source)
	compatibleVersions, err := g.compatibleVersions()
	if err != nil {
		return err
	}
	// Decide which versions to fetch: KDM-compatible and/or requested (GitHub-only) versions.
	var versionsToFetch []string
	if len(g.includeVersions) > 0 {
		// User requested specific versions: include those in KDM plus any requested but not in KDM (GitHub-only).
		kdmRequested := compatibleVersions[:0]
		for _, v := range compatibleVersions {
			if g.includeVersions[v] {
				kdmRequested = append(kdmRequested, v)
			}
		}
		versionsToFetch = kdmRequested
		for v := range g.includeVersions {
			if v == "" {
				continue
			}
			found := false
			for _, c := range compatibleVersions {
				if c == v {
					found = true
					break
				}
			}
			if !found {
				versionsToFetch = append(versionsToFetch, v)
				logrus.Infof("Including [%v] version %q from GitHub release (not in KDM for this Rancher version)", g.source, v)
			}
		}
		if len(versionsToFetch) == 0 {
			logrus.Infof("skipping [%v]: no requested versions (KDM or GitHub) to fetch", g.source)
			return nil
		}
	} else {
		// "All": only KDM-compatible versions.
		if len(compatibleVersions) == 0 {
			logrus.Infof("skipping image generation since no compatible releases "+
				"were found for version: %s", g.rancherVersion)
			return nil
		}
		versionsToFetch = compatibleVersions
	}

	rs := fmt.Sprintf("[%s-release(rancher)]", g.source)
	us := fmt.Sprintf("[%s-upgrade(rancher)]", g.source)
	for _, version := range versionsToFetch {
		g.versionSet[version] = true

		// Add upgrade images.
		upgradeImage := fmt.Sprintf("rancher/%s-upgrade:%s",
			g.source, strings.ReplaceAll(version, "+", "-"))
		if g.linuxImageSet[upgradeImage] == nil {
			g.linuxImageSet[upgradeImage] = make(map[string]bool)
		}
		g.linuxImageSet[upgradeImage][us] = true

		// Add system-agent-installer images.
		systemAgentInstallerImage := fmt.Sprintf(
			"%s%s:%s", "rancher/system-agent-installer-",
			g.source, strings.ReplaceAll(version, "+", "-"))
		if g.linuxImageSet[systemAgentInstallerImage] == nil {
			g.linuxImageSet[systemAgentInstallerImage] = make(map[string]bool)
		}
		g.linuxImageSet[systemAgentInstallerImage][rs] = true

		// Get linux images from GitHub Release.
		linuxImages, err := g.getLinuxExternalList(ctx, version)
		if err != nil {
			logrus.Errorf("Could not download linux images for %s [%s]: %v",
				g.source, version, err)
			return err
		}
		for _, img := range linuxImages {
			if g.linuxImageSet[img] == nil {
				g.linuxImageSet[img] = make(map[string]bool)
			}
			g.linuxImageSet[img][rs] = true
		}

		// Get windows images from GitHub Release.
		windowsImages, err := g.getWindowsExternalList(ctx, version)
		if err != nil {
			logrus.Errorf("Could not download windows images for %s [%s]: %v",
				g.source, version, err)
			return err
		}
		for _, img := range windowsImages {
			if g.windowsImageSet[img] == nil {
				g.windowsImageSet[img] = make(map[string]bool)
			}
			g.windowsImageSet[img][rs] = true
		}
	}

	return nil
}

func (g *k3sRKE2Getter) getLinuxExternalList(ctx context.Context, release string) ([]string, error) {
	var link string
	if g.imageListBaseURL != "" {
		// Rancher Prime: prime.ribs.rancher.io/k3s/{version}/k3s-images.txt or .../rke2/{version}/rke2-images-all.linux-amd64.txt
		encodedRelease := strings.ReplaceAll(release, "+", "%2B")
		switch g.source {
		case RKE2:
			link = fmt.Sprintf("%s/rke2/%s/rke2-images-all.linux-amd64.txt", g.imageListBaseURL, encodedRelease)
		case K3S:
			link = fmt.Sprintf("%s/k3s/%s/k3s-images.txt", g.imageListBaseURL, encodedRelease)
		default:
			return nil, fmt.Errorf("invalid image source: %v", g.source)
		}
	} else {
		switch g.source {
		case RKE2:
			link = fmt.Sprintf(RKE2ImageLinux, release)
		case K3S:
			link = fmt.Sprintf(K3SImageURL, release)
		default:
			return nil, fmt.Errorf("invalid image source: %v", g.source)
		}
	}
	return getImageListFromURL(ctx, g.insecureSkipVerify, link)
}

func (g *k3sRKE2Getter) getWindowsExternalList(ctx context.Context, release string) ([]string, error) {
	if g.source == K3S {
		return []string{}, nil // K3s does not support Windows
	}
	var link string
	if g.imageListBaseURL != "" {
		encodedRelease := strings.ReplaceAll(release, "+", "%2B")
		link = fmt.Sprintf("%s/rke2/%s/rke2-images.windows-amd64.txt", g.imageListBaseURL, encodedRelease)
	} else {
		link = fmt.Sprintf(RKE2ImageWindows, release)
	}
	return getImageListFromURL(ctx, g.insecureSkipVerify, link)
}

func getImageListFromURL(ctx context.Context, tlsVerify bool, link string) ([]string, error) {
	logrus.Infof("Get images from %q", link)
	client := &http.Client{
		Timeout: time.Second * 30,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !tlsVerify},
			Proxy:           http.ProxyFromEnvironment,
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return nil, fmt.Errorf("getImageListFromURL: %w", err)
	}
	resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
	if err != nil {
		return nil, fmt.Errorf("getImageListFromURL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get url %q: %v", link, resp.Status)
	}

	list := []string{}
	sc := bufio.NewScanner(resp.Body)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		l := sc.Text()
		if l == "" {
			continue
		}
		l = strings.TrimPrefix(l, "docker.io/")
		list = append(list, l)
	}
	return list, nil
}

func (g *k3sRKE2Getter) LinuxImageSet() map[string]map[string]bool {
	return g.linuxImageSet
}

func (g *k3sRKE2Getter) WindowsImageSet() map[string]map[string]bool {
	return g.windowsImageSet
}

func (g *k3sRKE2Getter) VersionSet() map[string]bool {
	return g.versionSet
}

func (g *k3sRKE2Getter) Source() ClusterType {
	return g.source
}

// filterDeprecatedVersions removes the deprecated k8s versions and only
// keeps the latest patch version of each minor release.
func filterDeprecatedVersions(versions []string) []string {
	if len(versions) == 0 {
		return versions
	}
	set := map[string]string{}
	for _, v := range versions {
		var err error
		v, err = utils.EnsureSemverValid(v)
		if err != nil {
			continue
		}
		mm := semver.MajorMinor(v)
		if set[mm] == "" {
			set[mm] = v
		} else {
			// Update the highest patch version
			if n, _ := utils.SemverCompare(v, set[mm]); n > 0 {
				set[mm] = v
			} else if n == 0 {
				if strings.Compare(v, set[mm]) > 0 {
					set[mm] = v
				}
			}
		}
	}
	filteredVersions := []string{}
	for _, v := range set {
		filteredVersions = append(filteredVersions, v)
	}
	slices.Sort(filteredVersions)
	return filteredVersions
}
