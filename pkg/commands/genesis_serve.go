// Package commands provides the genesis serve API for the Vue.js frontend.
package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

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
	Versions []string `json:"versions"`
}

type Step1DetailsJSON struct {
	KDMURL          string `json:"kdmUrl"`
	ImageListSource string `json:"imageListSource"`
}

// GenerateRequest is the request body for POST /api/generate.
type GenerateRequest struct {
	RancherVersion             string   `json:"rancherVersion"`
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
			mux.HandleFunc("/api/logs", handleLogs)
			if staticDir != "" {
				mux.Handle("/", http.FileServer(http.Dir(staticDir)))
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/rancher/rancher/tags?per_page=100", nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "request: "+err.Error())
		return
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := http.DefaultClient.Do(req)
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
	var tags []githubTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		writeErr(w, http.StatusInternalServerError, "decode: "+err.Error())
		return
	}
	var versions []string
	for _, t := range tags {
		name := strings.TrimSpace(t.Name)
		if strings.HasPrefix(name, "v") && semver.IsValid(name) {
			versions = append(versions, name)
		}
	}
	sort.Slice(versions, func(i, j int) bool { return semver.Compare(versions[i], versions[j]) > 0 })
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
		capJSON[string(k)] = ClusterVersionInfoJSON{Versions: v.Versions}
	}
	details := Step1DetailsJSON{
		KDMURL:          GetKDMURLForDisplay(cc.rancherVersion, cc.isRPMGC, cc.genesisOpts.dev),
		ImageListSource: GetImageListSourceForDisplay(cc.isRPMGC),
	}
	writeJSON(w, http.StatusOK, Step1OptionsResponse{
		HasRKE1:      hasRKE1,
		Capabilities: capJSON,
		Details:      details,
	})
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
	cc := newGenesisCmd()
	cc.genesisOpts.rancherVersion = req.RancherVersion
	if !strings.HasPrefix(cc.genesisOpts.rancherVersion, "v") {
		cc.genesisOpts.rancherVersion = "v" + cc.genesisOpts.rancherVersion
	}
	cc.genesisOpts.dev = isPreRelease(cc.genesisOpts.rancherVersion)
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
	roots, basicCharts, _, _, basicImageComponent, pastSelection := cc.buildGenesisTree()
	jobID := uuid.New().String()
	genesisJobsMu.Lock()
	genesisJobs[jobID] = &genesisJob{
		cc:                  cc,
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
	if err := cc.finish(); err != nil {
		writeErr(w, http.StatusInternalServerError, "finish: "+err.Error())
		return
	}
	data, err := os.ReadFile(cc.output)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "read output: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "attachment; filename=images.txt")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
