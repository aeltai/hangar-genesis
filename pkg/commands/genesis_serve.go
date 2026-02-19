// Package commands provides the genesis serve API for the Vue.js frontend.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cnrancher/hangar/pkg/image/scan"
	"github.com/cnrancher/hangar/pkg/rancher/kdmimages"
	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/google/uuid"
	"github.com/rancher/rke/types/kdm"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const genesisJobExpiry = 60 * time.Minute

type genesisJob struct {
	cc                  *genesisCmd
	created             time.Time
	roots               []treeNode
	basicCharts         []treeNode
	basicImageComponent map[string]string
	pastSelection       string
}

var (
	genesisJobs   = make(map[string]*genesisJob)
	genesisJobsMu sync.RWMutex
)

const (
	scanJobMaxImages = 50
	scanJobExpiry    = 30 * time.Minute
)

type scanJob struct {
	ID       string
	Status   string     // "running", "completed", "failed"
	Report   *scan.Report
	Error    string
	Created  time.Time
	done     chan struct{}
}

var (
	scanJobs   = make(map[string]*scanJob)
	scanJobsMu sync.RWMutex
)

const genesisLogMaxLines = 500

// genesisLogBuffer captures recent log lines for the /api/logs endpoint.
type genesisLogBuffer struct {
	mu    sync.RWMutex
	lines []string
	max   int
}

func (b *genesisLogBuffer) add(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, line)
	if len(b.lines) > b.max {
		b.lines = b.lines[len(b.lines)-b.max:]
	}
}

func (b *genesisLogBuffer) copy() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}

var genesisLogBuf = &genesisLogBuffer{max: genesisLogMaxLines}

// genesisLogHook is a logrus hook that writes entries to genesisLogBuf.
type genesisLogHook struct{}

func (genesisLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (genesisLogHook) Fire(e *logrus.Entry) error {
	msg, err := e.String()
	if err != nil {
		msg = e.Message
	}
	genesisLogBuf.add(msg)
	return nil
}

func init() {
	go func() {
		tick := time.NewTicker(15 * time.Minute)
		defer tick.Stop()
		for range tick.C {
			genesisJobsMu.Lock()
			for id, job := range genesisJobs {
				if time.Since(job.created) > genesisJobExpiry {
					delete(genesisJobs, id)
				}
			}
			genesisJobsMu.Unlock()
			scanJobsMu.Lock()
			for id, job := range scanJobs {
				if time.Since(job.Created) > scanJobExpiry {
					delete(scanJobs, id)
				}
			}
			scanJobsMu.Unlock()
		}
	}()
}

// TreeNodeJSON is the JSON representation of treeNode for API responses.
type TreeNodeJSON struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Kind     string         `json:"kind"`
	Count    int            `json:"count"`
	Children []TreeNodeJSON `json:"children,omitempty"`
}

func treeNodeToJSON(n treeNode) TreeNodeJSON {
	out := TreeNodeJSON{ID: n.Id, Label: n.Label, Kind: n.Kind, Count: n.Count}
	if len(n.Children) > 0 {
		out.Children = make([]TreeNodeJSON, 0, len(n.Children))
		for _, c := range n.Children {
			out.Children = append(out.Children, treeNodeToJSON(c))
		}
	}
	return out
}

func treeNodesToJSON(nodes []treeNode) []TreeNodeJSON {
	out := make([]TreeNodeJSON, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, treeNodeToJSON(n))
	}
	return out
}

// Step1OptionsResponse is the response for GET /api/step1-options.
type Step1OptionsResponse struct {
	HasRKE1      bool                              `json:"hasRKE1"`
	Capabilities map[string]ClusterVersionInfoJSON `json:"capabilities"`
	Details      Step1DetailsJSON                  `json:"details"`
}

type ClusterVersionInfoJSON struct {
	Versions []string          `json:"versions"`
	Sources  map[string]string `json:"sources,omitempty"` // version -> "kdm" | "github" | "both"
}

type Step1DetailsJSON struct {
	KDMURL          string `json:"kdmUrl"`
	ImageListSource string `json:"imageListSource"`
}

// GenerateRequest is the request body for POST /api/generate.
// If RancherVersions has multiple entries, the backend runs the generator for each and merges image lists.
type GenerateRequest struct {
	RancherVersion             string   `json:"rancherVersion"`             // single version (used when RancherVersions is empty)
	RancherVersions            []string `json:"rancherVersions,omitempty"`  // multiple versions: generate for each and merge
	IsRPMGC                    bool     `json:"isRPMGC"`
	IncludeAppCollectionCharts bool     `json:"includeAppCollectionCharts"`
	AppCollectionAPIUser       string   `json:"appCollectionAPIUser"`
	AppCollectionAPIPassword   string   `json:"appCollectionAPIPassword"`
	Distros                    []string `json:"distros"`
	CNI                        string   `json:"cni"`
	LoadBalancer               bool     `json:"loadBalancer"`
	LBK3sKlipper               bool     `json:"lbK3sKlipper"`
	LBK3sTraefik               bool     `json:"lbK3sTraefik"`
	LBRKE2Nginx                bool     `json:"lbRKE2Nginx"`
	LBRKE2Traefik              bool     `json:"lbRKE2Traefik"`
	IncludeWindows             bool     `json:"includeWindows"`
	K3sVersions                string   `json:"k3sVersions"`
	RKE2Versions               string   `json:"rke2Versions"`
	RKEVersions                string   `json:"rkeVersions"`
}

// GenerateResponse is the response for POST /api/generate.
type GenerateResponse struct {
	JobID               string            `json:"jobId"`
	Roots               []TreeNodeJSON    `json:"roots"`
	BasicCharts         []TreeNodeJSON    `json:"basicCharts"`
	BasicImageComponent map[string]string `json:"basicImageComponent"`
	PastSelection       string            `json:"pastSelection"`
}

// ExportRequest is the request body for POST /api/export.
type ExportRequest struct {
	JobID                string   `json:"jobId"`
	SelectedComponentIDs []string `json:"selectedComponentIDs"`
	ChartNames           []string `json:"chartNames"`
	SelectedImageRefs    []string `json:"selectedImageRefs"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newGenesisServeCmd adds the "genesis serve" subcommand to the genesis command.
func newGenesisServeCmd(parent *genesisCmd) {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run Genesis API server for the web UI",
		Long:  "Starts an HTTP server that serves the Genesis API (step1-options, generate, export) and optional static frontend.",
		RunE: func(c *cobra.Command, args []string) error {
			port, _ := c.Flags().GetString("port")
			if port == "" {
				port = "8080"
			}
			staticDir, _ := c.Flags().GetString("static")
			// Capture logrus output for GET /api/logs (e.g. loading screen).
			logrus.AddHook(genesisLogHook{})
			mux := http.NewServeMux()
			// Register API routes first so they take precedence over static "/" (important for Go < 1.22)
			mux.HandleFunc("/api/rancher-versions", handleRancherVersions)
			mux.HandleFunc("/api/step1-options", handleStep1Options)
			mux.HandleFunc("/api/generate", handleGenerate)
			mux.HandleFunc("/api/export", handleExport)
			mux.HandleFunc("/api/check-availability", handleCheckAvailability)
			mux.HandleFunc("/api/scan", handleScan)
			mux.HandleFunc("/api/scan/status/{id}", handleScanStatus)
			mux.HandleFunc("/api/scan/report/{id}", handleScanReport)
			mux.HandleFunc("/api/release-notes", handleReleaseNotes)
			mux.HandleFunc("/api/logs", handleLogs)
			if staticDir != "" {
				fs := http.FileServer(http.Dir(staticDir))
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/" || r.URL.Path == "/index.html" {
						w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
					}
					fs.ServeHTTP(w, r)
				})
			}
			addr := ":" + port
			logrus.Infof("Genesis API server listening on %s", addr)
			return http.ListenAndServe(addr, corsMiddleware(mux))
		},
	}
	cmd.Flags().String("port", "8080", "Port to listen on")
	cmd.Flags().String("static", "", "Directory to serve static frontend (e.g. frontend/dist)")
	parent.cmd.AddCommand(cmd)
}

// githubTagsResponse is a minimal struct for GitHub tags API response.
type githubTag struct {
	Name string `json:"name"`
}

func handleRancherVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	apiURL := "https://api.github.com/repos/rancher/rancher/releases?per_page=100"
	ghReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "request: "+err.Error())
		return
	}
	ghReq.Header.Set("Accept", "application/vnd.github.v3+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		ghReq.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(ghReq)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "fetch: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writeErr(w, http.StatusBadGateway, "GitHub API: "+resp.Status+" "+string(body))
		return
	}
	var releases []struct {
		TagName     string `json:"tag_name"`
		Prerelease  bool   `json:"prerelease"`
		Draft       bool   `json:"draft"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		writeErr(w, http.StatusInternalServerError, "decode: "+err.Error())
		return
	}
	includeRC := r.URL.Query().Get("includeRC") == "true"
	type versionInfo struct {
		Version string `json:"version"`
		Date    string `json:"date"`
	}
	var versions []versionInfo
	for _, rel := range releases {
		if rel.Draft {
			continue
		}
		name := strings.TrimSpace(rel.TagName)
		if !strings.HasPrefix(name, "v") || !semver.IsValid(name) {
			continue
		}
		if !includeRC && (rel.Prerelease || isPreRelease(name)) {
			continue
		}
		date := ""
		if rel.PublishedAt != "" {
			if t, err := time.Parse(time.RFC3339, rel.PublishedAt); err == nil {
				date = t.Format("2006-01-02")
			}
		}
		versions = append(versions, versionInfo{Version: name, Date: date})
	}
	sort.Slice(versions, func(i, j int) bool { return semver.Compare(versions[i].Version, versions[j].Version) > 0 })
	writeJSON(w, http.StatusOK, map[string]interface{}{"versions": versions})
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	lines := genesisLogBuf.copy()
	writeJSON(w, http.StatusOK, map[string]interface{}{"lines": lines})
}

func handleStep1Options(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	rancherVersion := r.URL.Query().Get("rancher")
	if rancherVersion == "" {
		writeErr(w, http.StatusBadRequest, "rancher query parameter required")
		return
	}
	if !strings.HasPrefix(rancherVersion, "v") {
		rancherVersion = "v" + rancherVersion
	}
	cc := &genesisCmd{genesisOpts: &genesisOpts{
		rancherVersion:      rancherVersion,
		dev:                 isPreRelease(rancherVersion),
		kdmRemoveDeprecated: true,
	}}
	if err := cc.setupFlags(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	kdmBytes, err := cc.loadKDMData(signalContext)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load KDM: "+err.Error())
		return
	}
	data, err := kdm.FromData(kdmBytes)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "parse KDM: "+err.Error())
		return
	}
	minKube := ""
	if cc.minKubeVersion != "" {
		minKube = semver.MajorMinor(cc.minKubeVersion)
	} else {
		minKube = "v1.30.0"
	}
	capabilities, err := kdmimages.InspectClusterVersions(
		cc.rancherVersion, minKube, cc.kdmRemoveDeprecated, data)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "inspect versions: "+err.Error())
		return
	}
	hasRKE1 := false
	if ok, _ := utils.SemverCompare(cc.rancherVersion, "v2.12.0-0"); ok < 0 {
		hasRKE1 = true
	}
	capJSON := make(map[string]ClusterVersionInfoJSON)
	for k, v := range capabilities {
		src := make(map[string]string, len(v.Versions))
		for _, ver := range v.Versions {
			src[ver] = "kdm"
		}
		capJSON[string(k)] = ClusterVersionInfoJSON{Versions: v.Versions, Sources: src}
	}
	details := Step1DetailsJSON{
		KDMURL:          GetKDMURLForDisplay(cc.rancherVersion, cc.isRPMGC, cc.genesisOpts.dev),
		ImageListSource: GetImageListSourceForDisplay(cc.isRPMGC),
	}
	includeRC := r.URL.Query().Get("includeRC") == "true"
	includeGitHubVersions := r.URL.Query().Get("includeGitHubVersions") == "true"
	if includeGitHubVersions {
		mergeGitHubVersions(r.Context(), capJSON, includeRC)
	}

	writeJSON(w, http.StatusOK, Step1OptionsResponse{
		HasRKE1:      hasRKE1,
		Capabilities: capJSON,
		Details:      details,
	})
}

// githubRelease is a minimal struct for GitHub releases API.
type githubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

// mergeGitHubVersions fetches recent stable (and optionally RC) releases from
// K3s/RKE2 GitHub and merges them into the capabilities map. Versions already
// present from KDM are marked "both"; new versions are marked "github".
func mergeGitHubVersions(ctx context.Context, caps map[string]ClusterVersionInfoJSON, includeRC bool) {
	type repoInfo struct {
		key   string
		owner string
		repo  string
	}
	repos := []repoInfo{
		{"k3s", "k3s-io", "k3s"},
		{"rke2", "rancher", "rke2"},
	}
	for _, ri := range repos {
		cap, ok := caps[ri.key]
		if !ok {
			continue
		}
		ghVersions := fetchGitHubReleases(ctx, ri.owner, ri.repo, includeRC)
		if len(ghVersions) == 0 {
			continue
		}
		if cap.Sources == nil {
			cap.Sources = make(map[string]string)
		}
		existing := make(map[string]bool, len(cap.Versions))
		for _, v := range cap.Versions {
			existing[v] = true
		}
		for _, gh := range ghVersions {
			if existing[gh] {
				cap.Sources[gh] = "both"
			} else {
				cap.Versions = append(cap.Versions, gh)
				cap.Sources[gh] = "github"
			}
		}
		caps[ri.key] = cap
	}
}

func fetchGitHubReleases(ctx context.Context, owner, repo string, includeRC bool) []string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	apiURL := "https://api.github.com/repos/" + owner + "/" + repo + "/releases?per_page=50"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		logrus.Debugf("fetchGitHubReleases: %v", err)
		return nil
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.Debugf("fetchGitHubReleases: %v", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil
	}
	var versions []string
	for _, r := range releases {
		if r.Draft {
			continue
		}
		if r.Prerelease && !includeRC {
			continue
		}
		tag := strings.TrimSpace(r.TagName)
		if tag == "" {
			continue
		}
		versions = append(versions, tag)
	}
	return versions
}

// mergeImageMaps merges b into a (union of keys; inner maps merged).
func mergeImageMaps(a, b map[string]map[string]bool) {
	for img, sources := range b {
		if a[img] == nil {
			a[img] = make(map[string]bool)
		}
		for src := range sources {
			a[img][src] = true
		}
	}
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	versions := req.RancherVersions
	if len(versions) == 0 && req.RancherVersion != "" {
		versions = []string{req.RancherVersion}
	}
	if len(versions) == 0 {
		writeErr(w, http.StatusBadRequest, "rancherVersion or rancherVersions required")
		return
	}
	// Normalize version strings
	for i := range versions {
		if !strings.HasPrefix(versions[i], "v") {
			versions[i] = "v" + versions[i]
		}
	}

	var firstCC *genesisCmd
	var mergedLinux, mergedWindows map[string]map[string]bool

	for _, rv := range versions {
		cc := newGenesisCmd()
		cc.genesisOpts.rancherVersion = rv
		cc.genesisOpts.dev = isPreRelease(rv)
		cc.isRPMGC = req.IsRPMGC
		cc.includeAppCollectionCharts = req.IncludeAppCollectionCharts
		cc.appCollectionAPIUser = req.AppCollectionAPIUser
		cc.appCollectionAPIPassword = req.AppCollectionAPIPassword
		cc.genesisOpts.components = strings.Join(req.Distros, ",")
		cc.genesisOpts.interactiveSelectedCNI = req.CNI
		cc.genesisOpts.interactiveIncludeLB = req.LoadBalancer
		cc.genesisOpts.interactiveLBK3sKlipper = req.LBK3sKlipper
		cc.genesisOpts.interactiveLBK3sTraefik = req.LBK3sTraefik
		cc.genesisOpts.interactiveLBRKE2Nginx = req.LBRKE2Nginx
		cc.genesisOpts.interactiveLBRKE2Traefik = req.LBRKE2Traefik
		cc.genesisOpts.interactiveIncludeWindows = req.IncludeWindows
		cc.genesisOpts.k3sVersions = req.K3sVersions
		cc.genesisOpts.rke2Versions = req.RKE2Versions
		cc.genesisOpts.rkeVersions = req.RKEVersions
		cc.genesisOpts.interactive = false
		cc.genesisOpts.configFile = ""

		if err := cc.setupFlags(); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := cc.parseComponentFlags(); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := cc.prepareGenerator(); err != nil {
			writeErr(w, http.StatusInternalServerError, "prepare generator: "+err.Error())
			return
		}
		if err := cc.run(signalContext); err != nil {
			writeErr(w, http.StatusInternalServerError, "run generator: "+err.Error())
			return
		}
		if firstCC == nil {
			firstCC = cc
			mergedLinux = make(map[string]map[string]bool)
			mergedWindows = make(map[string]map[string]bool)
			for k, v := range cc.generator.LinuxImages {
				mergedLinux[k] = make(map[string]bool)
				for s := range v {
					mergedLinux[k][s] = true
				}
			}
			for k, v := range cc.generator.WindowsImages {
				mergedWindows[k] = make(map[string]bool)
				for s := range v {
					mergedWindows[k][s] = true
				}
			}
		} else {
			mergeImageMaps(mergedLinux, cc.generator.LinuxImages)
			mergeImageMaps(mergedWindows, cc.generator.WindowsImages)
		}
	}

	firstCC.generator.LinuxImages = mergedLinux
	firstCC.generator.WindowsImages = mergedWindows
	if len(versions) > 1 {
		firstCC.genesisOpts.rancherVersionsList = versions
		firstCC.genesisOpts.rancherVersion = versions[0] + " + " + strings.Join(versions[1:], ", ")
	}

	roots, basicCharts, _, _, basicImageComponent, pastSelection := firstCC.buildGenesisTree()
	jobID := uuid.New().String()
	genesisJobsMu.Lock()
	genesisJobs[jobID] = &genesisJob{
		cc:                  firstCC,
		created:             time.Now(),
		roots:               roots,
		basicCharts:         basicCharts,
		basicImageComponent: basicImageComponent,
		pastSelection:       pastSelection,
	}
	genesisJobsMu.Unlock()
	writeJSON(w, http.StatusOK, GenerateResponse{
		JobID:               jobID,
		Roots:               treeNodesToJSON(roots),
		BasicCharts:         treeNodesToJSON(basicCharts),
		BasicImageComponent: basicImageComponent,
		PastSelection:       pastSelection,
	})
}

func handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	genesisJobsMu.RLock()
	job, ok := genesisJobs[req.JobID]
	genesisJobsMu.RUnlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "job not found or expired")
		return
	}
	cc := job.cc
	cc.interactiveSelectedComponentIDs = req.SelectedComponentIDs
	cc.interactiveSelectedChartNames = req.ChartNames
	cc.interactiveSelectedImageRefs = req.SelectedImageRefs
	cc.autoYes = true // non-interactive: never prompt for overwrite (e.g. *-versions.txt)
	dir, err := os.MkdirTemp("", "genesis-export-*")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "temp dir: "+err.Error())
		return
	}
	defer os.RemoveAll(dir)
	cc.output = filepath.Join(dir, "images.txt")
	if cc.interactiveIncludeWindows {
		cc.outputWindows = filepath.Join(dir, "images-windows.txt")
	}
	if err := cc.finish(); err != nil {
		writeErr(w, http.StatusInternalServerError, "finish: "+err.Error())
		return
	}
	data, err := os.ReadFile(cc.output)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "read output: "+err.Error())
		return
	}
	if cc.interactiveIncludeWindows && cc.outputWindows != "" {
		winData, err := os.ReadFile(cc.outputWindows)
		if err == nil && len(winData) > 0 {
			if len(data) > 0 && data[len(data)-1] != '\n' {
				data = append(data, '\n')
			}
			data = append(data, winData...)
		}
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "attachment; filename=images.txt")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleScan starts an async Trivy scan. POST body: { "images": ["ref1", ...] }. Returns { "scanJobId": "uuid" }.
func handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req struct {
		Images []string `json:"images"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Images) == 0 {
		writeErr(w, http.StatusBadRequest, "images required")
		return
	}
	if len(req.Images) > scanJobMaxImages {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("too many images (max %d)", scanJobMaxImages))
		return
	}
	scanJobID := uuid.New().String()
	job := &scanJob{
		ID:      scanJobID,
		Status:  "running",
		Created: time.Now(),
		done:    make(chan struct{}),
	}
	scanJobsMu.Lock()
	scanJobs[scanJobID] = job
	scanJobsMu.Unlock()
	images := make([]string, len(req.Images))
	copy(images, req.Images)
	logrus.Infof("Scan started (job %s, %d images)", scanJobID, len(images))
	go func() {
		defer close(job.done)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		report, err := RunScanWithOptions(ctx, images, RunScanOptions{
			InsecureSkipTLS: true,
			Jobs:            2,
			Timeout:         10 * time.Minute,
		})
		scanJobsMu.Lock()
		defer scanJobsMu.Unlock()
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			if report != nil {
				job.Report = report
			}
			logrus.Warnf("Scan failed (job %s): %v", scanJobID, err)
			return
		}
		job.Status = "completed"
		job.Report = report
		logrus.Infof("Scan completed (job %s)", scanJobID)
	}()
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, http.StatusOK, map[string]string{"scanJobId": scanJobID})
}

// handleScanStatus returns status for a scan job. GET /api/scan/status/{id}.
func handleScanStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id required")
		return
	}
	scanJobsMu.RLock()
	job, ok := scanJobs[id]
	scanJobsMu.RUnlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "scan job not found or expired")
		return
	}
	resp := map[string]interface{}{"status": job.Status}
	if job.Error != "" {
		resp["error"] = job.Error
	}
	if job.Status == "completed" && job.Report != nil {
		var critical, high, medium, low int
		for _, result := range job.Report.Results {
			for _, img := range result.Images {
				for _, v := range img.Vulnerabilities {
					switch v.SeverityString {
					case "CRITICAL":
						critical++
					case "HIGH":
						high++
					case "MEDIUM":
						medium++
					case "LOW":
						low++
					}
				}
			}
		}
		resp["summary"] = map[string]int{"critical": critical, "high": high, "medium": medium, "low": low}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleScanReport returns the scan report CSV. GET /api/scan/report/{id}.
func handleScanReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id required")
		return
	}
	scanJobsMu.RLock()
	job, ok := scanJobs[id]
	scanJobsMu.RUnlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "scan job not found or expired")
		return
	}
	if job.Status != "completed" || job.Report == nil {
		writeErr(w, http.StatusConflict, "scan not completed yet")
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=scan-report.csv")
	w.WriteHeader(http.StatusOK)
	_ = job.Report.WriteCSV(w)
}

// handleReleaseNotes fetches release notes (changelog) from the GitHub Releases API.
// Supports rancher/rancher (Rancher versions), rancher/rke2, and rancher/k3s.
// GET /api/release-notes?repo=rancher/rancher&tag=v2.13.1
func handleReleaseNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	repo := r.URL.Query().Get("repo")
	tag := r.URL.Query().Get("tag")
	if repo == "" || tag == "" {
		writeErr(w, http.StatusBadRequest, "repo and tag required")
		return
	}

	apiURL := "https://api.github.com/repos/" + repo + "/releases/tags/" + tag
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := client.Do(req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "github: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		writeErr(w, resp.StatusCode, "github "+resp.Status+": "+string(body))
		return
	}

	var release struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		HTMLURL     string `json:"html_url"`
		Prerelease  bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		writeErr(w, http.StatusBadGateway, "decode: "+err.Error())
		return
	}

	// Parse chart versions table and changelog from the markdown body
	charts := parseChartsTable(release.Body)
	changelog := parseChangelog(release.Body)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tag":         release.TagName,
		"name":        release.Name,
		"publishedAt": release.PublishedAt,
		"url":         release.HTMLURL,
		"prerelease":  release.Prerelease,
		"charts":      charts,
		"changelog":   changelog,
		"body":        release.Body,
	})
}

// parseChartsTable extracts chart name→version pairs from the markdown release body.
func parseChartsTable(body string) []map[string]string {
	var charts []map[string]string
	lines := strings.Split(body, "\n")
	inTable := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Charts Versions") {
			inTable = true
			continue
		}
		if inTable && strings.HasPrefix(line, "|") {
			cols := strings.Split(line, "|")
			if len(cols) < 3 {
				continue
			}
			name := strings.TrimSpace(cols[1])
			verRaw := strings.TrimSpace(cols[2])
			if name == "" || name == "Component" || strings.HasPrefix(name, "-") {
				continue
			}
			// Extract version from markdown link [version](url) or plain text
			ver := verRaw
			if i := strings.Index(verRaw, "["); i >= 0 {
				if j := strings.Index(verRaw[i:], "]"); j > 0 {
					ver = verRaw[i+1 : i+j]
				}
			}
			charts = append(charts, map[string]string{"name": name, "version": ver})
		}
		if inTable && !strings.HasPrefix(line, "|") && line != "" && !strings.HasPrefix(line, "#") {
			inTable = false
		}
	}
	return charts
}

// parseChangelog extracts the "Changes since" section from the markdown.
func parseChangelog(body string) []string {
	var changes []string
	lines := strings.Split(body, "\n")
	inChanges := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "Changes since") {
			inChanges = true
			continue
		}
		if inChanges {
			if strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "| ") {
				break
			}
			if strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "-") {
				entry := strings.TrimLeft(trimmed, "*- ")
				if entry != "" {
					changes = append(changes, entry)
				}
			}
		}
	}
	return changes
}

// handleCheckAvailability checks if selected images are accessible in their registry.
func handleCheckAvailability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req struct {
		Images []string `json:"images"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Images) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{"results": map[string]interface{}{}})
		return
	}

	type result struct {
		img    string
		status string // "ok", "not_found", "error"
		detail string
	}

	results := make([]result, len(req.Images))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // concurrency limit

	for i, img := range req.Images {
		wg.Add(1)
		go func(idx int, image string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			status, detail := checkImageAvailability(image)
			results[idx] = result{img: image, status: status, detail: detail}
		}(i, img)
	}
	wg.Wait()

	out := make(map[string]interface{})
	for _, res := range results {
		out[res.img] = map[string]string{"status": res.status, "detail": res.detail}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"results": out})
}

// checkImageAvailability checks if an image exists in its registry using Docker Registry HTTP API v2.
func checkImageAvailability(image string) (status, detail string) {
	registry := "registry-1.docker.io"
	authService := "registry.docker.io"
	repo := image
	tag := "latest"

	// Parse image ref: registry/repo:tag
	if i := strings.LastIndex(repo, ":"); i > 0 {
		tag = repo[i+1:]
		repo = repo[:i]
	}

	// If no explicit registry prefix, assume docker.io
	parts := strings.SplitN(repo, "/", 3)
	if len(parts) == 3 && strings.Contains(parts[0], ".") {
		registry = parts[0]
		repo = parts[1] + "/" + parts[2]
		authService = ""
	} else if len(parts) == 2 && strings.Contains(parts[0], ".") {
		registry = parts[0]
		repo = parts[1]
		authService = ""
	}

	// For docker.io, strip "docker.io/" prefix
	repo = strings.TrimPrefix(repo, "docker.io/")

	// Add "library/" for official images
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}

	client := &http.Client{Timeout: 15 * time.Second}

	// Get auth token (Docker Hub)
	token := ""
	if registry == "registry-1.docker.io" {
		tokenURL := "https://auth.docker.io/token?service=" + authService + "&scope=repository:" + repo + ":pull"
		resp, err := client.Get(tokenURL)
		if err != nil {
			return "error", "auth: " + err.Error()
		}
		defer resp.Body.Close()
		var tokenResp struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			return "error", "auth decode: " + err.Error()
		}
		token = tokenResp.Token
	}

	// HEAD request to check manifest
	manifestURL := "https://" + registry + "/v2/" + repo + "/manifests/" + tag
	req, _ := http.NewRequest("HEAD", manifestURL, nil)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.oci.image.index.v1+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "error", err.Error()
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		return "ok", ""
	case 401:
		return "not_found", "unauthorized (image may not exist or requires auth)"
	case 404:
		return "not_found", "image not found in registry"
	default:
		return "error", "HTTP " + resp.Status
	}
}
