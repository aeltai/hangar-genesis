package listgenerator

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cnrancher/hangar/pkg/rancher/chartimages"
	"github.com/cnrancher/hangar/pkg/rancher/kdmimages"
	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/rancher/rke/types/kdm"
	"github.com/sirupsen/logrus"
)

type GeneratorOption struct {
	RancherVersion string
	MinKubeVersion string

	ChartsPaths map[string]chartimages.ChartRepoType // map[url]type
	ChartURLs   map[string]struct {
		Type   chartimages.ChartRepoType
		Branch string
	}

	KDMPath string // The path of KDM data.json file.
	KDMURL  string // The remote URL of KDM data.json.

	// ImageListBaseURL when set (e.g. Rancher Prime https://prime.ribs.rancher.io) uses that base for
	// K3s/RKE2 image list URLs and enables fetching rancher-images.txt from {base}/rancher/{version}/rancher-images.txt
	ImageListBaseURL string

	InsecureSkipTLS     bool
	RemoveDeprecatedKDM bool

	// IncludeClusterTypes limits which cluster types are included (K3S, RKE2, RKE). Empty = all.
	IncludeClusterTypes []kdmimages.ClusterType
	IncludeK3sVersions  []string
	IncludeRKE2Versions []string
	IncludeRKE1Versions []string

	IncludeChartImages bool
	IncludeChartNames  []string

	AppCollectionCharts []string
	AppCollectionImages []string
}

// Generator is a generator to generate image list from charts, KDM data, etc.
type Generator struct {
	rancherVersion string // Rancher version, should be va.b.c
	minKubeVersion string // Minimum RKE1 kube verision, should be va.b.c

	chartsPaths map[string]chartimages.ChartRepoType // map[url]type
	chartURLs   map[string]struct {
		Type   chartimages.ChartRepoType
		Branch string
	}

	kdmPath string
	kdmURL  string
	imageListBaseURL   string

	insecureSkipTLS     bool
	removeDeprecatedKDM bool

	includeClusterTypes map[kdmimages.ClusterType]bool
	includeK3sVersions  map[string]bool
	includeRKE2Versions map[string]bool
	includeRKE1Versions map[string]bool
	includeChartImages  bool
	includeChartNames   map[string]bool
	appCollectionCharts []string
	appCollectionImages []string

	// All generated images, map[image]map[source]true
	LinuxImages   map[string]map[string]bool
	WindowsImages map[string]map[string]bool

	RKE1LinuxImages   map[string]map[string]bool
	RKE2LinuxImages   map[string]map[string]bool
	K3sLinuxImages    map[string]map[string]bool
	RKE2WindowsImages map[string]map[string]bool

	RKE1Versions map[string]bool
	RKE2Versions map[string]bool
	K3sVersions  map[string]bool
}

func NewGenerator(o *GeneratorOption) (*Generator, error) {
	if o.RancherVersion == "" {
		return nil, fmt.Errorf("invalid rancher version")
	}
	rancherVersion, err := utils.EnsureSemverValid(o.RancherVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid rancher version: %v", o.RancherVersion)
	}
	if o.ChartURLs == nil && o.ChartsPaths == nil &&
		o.KDMPath == "" && o.KDMURL == "" &&
		len(o.AppCollectionCharts) == 0 && len(o.AppCollectionImages) == 0 {
		return nil, fmt.Errorf("no input source provided")
	}

	includeClusterTypes := make(map[kdmimages.ClusterType]bool)
	for _, t := range o.IncludeClusterTypes {
		includeClusterTypes[t] = true
	}
	includeK3sVersions := make(map[string]bool)
	for _, v := range o.IncludeK3sVersions {
		includeK3sVersions[v] = true
	}
	includeRKE2Versions := make(map[string]bool)
	for _, v := range o.IncludeRKE2Versions {
		includeRKE2Versions[v] = true
	}
	includeRKE1Versions := make(map[string]bool)
	for _, v := range o.IncludeRKE1Versions {
		includeRKE1Versions[v] = true
	}
	includeChartNames := make(map[string]bool)
	for _, name := range o.IncludeChartNames {
		includeChartNames[name] = true
	}

		g := &Generator{
		rancherVersion:      rancherVersion,
		minKubeVersion:      o.MinKubeVersion,
		chartsPaths:         o.ChartsPaths,
		chartURLs:           o.ChartURLs,
		kdmPath:             o.KDMPath,
		kdmURL:              o.KDMURL,
		imageListBaseURL:   o.ImageListBaseURL,
		insecureSkipTLS:     o.InsecureSkipTLS,
		removeDeprecatedKDM: o.RemoveDeprecatedKDM,

		includeClusterTypes: includeClusterTypes,
		includeK3sVersions:  includeK3sVersions,
		includeRKE2Versions: includeRKE2Versions,
		includeRKE1Versions: includeRKE1Versions,
		includeChartImages:  o.IncludeChartImages,
		includeChartNames:   includeChartNames,
		appCollectionCharts: o.AppCollectionCharts,
		appCollectionImages: o.AppCollectionImages,

		LinuxImages:       make(map[string]map[string]bool),
		WindowsImages:     make(map[string]map[string]bool),
		K3sLinuxImages:    make(map[string]map[string]bool),
		K3sVersions:       make(map[string]bool),
		RKE1LinuxImages:   make(map[string]map[string]bool),
		RKE1Versions:      make(map[string]bool),
		RKE2LinuxImages:   make(map[string]map[string]bool),
		RKE2WindowsImages: make(map[string]map[string]bool),
		RKE2Versions:      make(map[string]bool),
	}
	return g, nil
}

func (g *Generator) Run(ctx context.Context) error {
	if err := g.generateFromChartPaths(ctx); err != nil {
		return err
	}
	if err := g.generateFromChartURLs(ctx); err != nil {
		return err
	}
	if err := g.generateFromKDMPath(ctx); err != nil {
		return err
	}
	if err := g.generateFromKDMURL(ctx); err != nil {
		return err
	}
	if err := g.generateFromPrimeRancherImages(ctx); err != nil {
		return err
	}
	if err := g.generateFromAppCollection(ctx); err != nil {
		return err
	}
	return nil
}

func (g *Generator) generateFromChartPaths(ctx context.Context) error {
	if len(g.chartsPaths) == 0 {
		return nil
	}
	for path := range g.chartsPaths {
		c := chartimages.Chart{
			RancherVersion: g.rancherVersion,
			OS:             chartimages.Linux,
			Type:           g.chartsPaths[path],
			Path:           path,
		}
		if err := c.FetchImages(ctx); err != nil {
			return err
		}
		for image := range c.ImageSet {
			for source := range c.ImageSet[image] {
				utils.AddSourceToImage(g.LinuxImages, image, source)
			}
		}
		// fetch windows images
		c.OS = chartimages.Windows
		c.ImageSet = make(map[string]map[string]bool)
		if err := c.FetchImages(ctx); err != nil {
			return err
		}
		for image := range c.ImageSet {
			for source := range c.ImageSet[image] {
				utils.AddSourceToImage(g.WindowsImages, image, source)
			}
		}
	}
	return nil
}

func (g *Generator) generateFromChartURLs(ctx context.Context) error {
	if len(g.chartURLs) == 0 {
		return nil
	}
	for url := range g.chartURLs {
		c := chartimages.Chart{
			RancherVersion:  g.rancherVersion,
			OS:              chartimages.Linux,
			Type:            g.chartURLs[url].Type,
			Branch:          g.chartURLs[url].Branch,
			URL:             url,
			InsecureSkipTLS: g.insecureSkipTLS,
		}
		if err := c.FetchImages(ctx); err != nil {
			return err
		}
		for image := range c.ImageSet {
			if chartimages.IgnoreChartImages[image] {
				continue
			}
			for source := range c.ImageSet[image] {
				utils.AddSourceToImage(g.LinuxImages, image, source)
			}
		}
		// fetch windows images
		c.OS = chartimages.Windows
		c.ImageSet = make(map[string]map[string]bool)
		if err := c.FetchImages(ctx); err != nil {
			return err
		}
		for image := range c.ImageSet {
			if chartimages.IgnoreChartImages[image] {
				continue
			}
			for source := range c.ImageSet[image] {
				utils.AddSourceToImage(g.WindowsImages, image, source)
			}
		}
	}
	return nil
}

func (g *Generator) generateFromKDMPath(ctx context.Context) error {
	if g.kdmPath == "" {
		return nil
	}
	b, err := os.ReadFile(g.kdmPath)
	if err != nil {
		return err
	}
	return g.generateFromKDMData(ctx, b)
}

func (g *Generator) generateFromKDMURL(ctx context.Context) error {
	if g.kdmURL == "" {
		return nil
	}
	logrus.Infof("Get KDM data from URL: %q", g.kdmURL)

	client := &http.Client{
		Timeout: time.Second * 15,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: g.insecureSkipTLS,
			},
			Proxy: http.ProxyFromEnvironment,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.kdmURL, nil)
	if err != nil {
		return fmt.Errorf("generateFromKDMURL: %w", err)
	}
	resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
	if err != nil {
		return fmt.Errorf("generateFromKDMURL: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("generateFromKDMURL: %w", err)
	}
	return g.generateFromKDMData(ctx, b)
}

func (g *Generator) generateFromKDMData(ctx context.Context, b []byte) error {
	data, err := kdm.FromData(b)
	if err != nil {
		return fmt.Errorf("generateFromKDMData: %w", err)
	}
	clusters := []kdmimages.ClusterType{
		kdmimages.K3S,
		kdmimages.RKE2,
	}
	if ok, _ := utils.SemverCompare(g.rancherVersion, "v2.12.0-0"); ok < 0 {
		clusters = append(clusters, kdmimages.RKE)
	}
	// When config/TUI limited distros, only fetch those cluster types
	if len(g.includeClusterTypes) > 0 {
		filtered := clusters[:0]
		for _, t := range clusters {
			if g.includeClusterTypes[t] {
				filtered = append(filtered, t)
			}
		}
		clusters = filtered
	}
	for _, t := range clusters {
		opts := &kdmimages.GetterOptions{
			Type:              t,
			RancherVersion:    g.rancherVersion,
			MinKubeVersion:    g.minKubeVersion,
			KDMData:           data,
			ImageListBaseURL:  g.imageListBaseURL,
			InsecureSkipTLS:   g.insecureSkipTLS,
			RemoveDeprecated:  g.removeDeprecatedKDM,
		}
		switch t {
		case kdmimages.K3S:
			for v := range g.includeK3sVersions {
				opts.IncludeVersions = append(opts.IncludeVersions, v)
			}
		case kdmimages.RKE2:
			for v := range g.includeRKE2Versions {
				opts.IncludeVersions = append(opts.IncludeVersions, v)
			}
		case kdmimages.RKE:
			for v := range g.includeRKE1Versions {
				opts.IncludeVersions = append(opts.IncludeVersions, v)
			}
		}
		getter, err := kdmimages.NewGetter(opts)
		if err != nil {
			return err
		}

		if err = getter.Get(ctx); err != nil {
			return err
		}
		utils.MergeImageSourceSet(g.LinuxImages, getter.LinuxImageSet())
		utils.MergeImageSourceSet(g.WindowsImages, getter.WindowsImageSet())
		// Merge sets
		switch getter.Source() {
		case kdmimages.RKE:
			utils.MergeSets(g.RKE1Versions, getter.VersionSet())
			utils.MergeImageSourceSet(g.RKE1LinuxImages, getter.LinuxImageSet())
		case kdmimages.RKE2:
			utils.MergeSets(g.RKE2Versions, getter.VersionSet())
			utils.MergeImageSourceSet(g.RKE2LinuxImages, getter.LinuxImageSet())
			// RKE2 supports Windows
			utils.MergeImageSourceSet(g.RKE2WindowsImages, getter.WindowsImageSet())
		case kdmimages.K3S:
			utils.MergeSets(g.K3sVersions, getter.VersionSet())
			utils.MergeImageSourceSet(g.K3sLinuxImages, getter.LinuxImageSet())
		}
	}
	return nil
}

// generateFromPrimeRancherImages fetches rancher-images.txt from Prime base URL when set
// (e.g. https://prime.ribs.rancher.io/rancher/v2.13.2/rancher-images.txt) and merges into LinuxImages.
func (g *Generator) generateFromPrimeRancherImages(ctx context.Context) error {
	if g.imageListBaseURL == "" {
		return nil
	}
	version := strings.TrimPrefix(g.rancherVersion, "v")
	url := fmt.Sprintf("%s/rancher/v%s/rancher-images.txt", strings.TrimSuffix(g.imageListBaseURL, "/"), version)
	logrus.Infof("Get Rancher Prime images from %q", url)
	client := &http.Client{
		Timeout: 90 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: g.insecureSkipTLS},
			Proxy:           http.ProxyFromEnvironment,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("prime rancher-images: %w", err)
	}
	resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
	if err != nil {
		return fmt.Errorf("prime rancher-images: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prime rancher-images: %s returned %d", url, resp.StatusCode)
	}
	const source = "[prime-rancher-images]"
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "docker.io/")
		if g.LinuxImages[line] == nil {
			g.LinuxImages[line] = make(map[string]bool)
		}
		g.LinuxImages[line][source] = true
	}
	return sc.Err()
}

func (g *Generator) generateFromAppCollection(ctx context.Context) error {
	if len(g.appCollectionCharts) == 0 && len(g.appCollectionImages) == 0 {
		return nil
	}
	const source = "[app-collection]"
	for _, imageRef := range g.appCollectionImages {
		if imageRef == "" {
			continue
		}
		if g.LinuxImages[imageRef] == nil {
			g.LinuxImages[imageRef] = make(map[string]bool)
		}
		g.LinuxImages[imageRef][source] = true
	}
	// OCI chart refs (oci://dp.apps.rancher.io/charts/...) require helm pull;
	// chartimages currently supports only path and git URL. Skip chart image extraction for now.
	if len(g.appCollectionCharts) > 0 {
		logrus.Debugf("App Collection chart refs (%d) not yet supported for image extraction", len(g.appCollectionCharts))
	}
	return nil
}
