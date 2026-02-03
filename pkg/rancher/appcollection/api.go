package appcollection

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	// AppsAPIBase is the base URL for the Rancher Applications Metadata Service (Production).
	// See: https://api.apps.rancher.io/v1 (Metadata Service, GET /applications).
	AppsAPIBase = "https://api.apps.rancher.io/v1"
	// PackagingFormatHelmChart is the canonical packaging_format value for Helm charts (API enum: RPM, CONTAINER, HELM_CHART, HELM, CHART).
	PackagingFormatHelmChart = "HELM_CHART"
	// PackagingFormatContainer is the packaging_format value for container images.
	PackagingFormatContainer = "CONTAINER"
	// ChartsRegistry is the OCI registry for Application Collection charts.
	ChartsRegistry = "dp.apps.rancher.io"
	// ContainersRegistry is the registry prefix for Application Collection container images.
	ContainersRegistry = "dp.apps.rancher.io/containers"
	// DefaultPageSize is the number of applications per page (API default 20; we use 100 to reduce requests).
	DefaultPageSize = 100
)

// ApplicationsResponse is the JSON response from GET /v1/applications (paged).
// See Metadata Service API: page_number, page_size, total_size, total_pages.
type ApplicationsResponse struct {
	Items      []ApplicationItem `json:"items"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalSize  int               `json:"total_size"`
	TotalPages int               `json:"total_pages"`
}

// ApplicationItem represents one application in the API response.
type ApplicationItem struct {
	Name            string `json:"name"`
	SlugName        string `json:"slug_name"`
	PackagingFormat string `json:"packaging_format"` // RPM, CONTAINER, HELM_CHART, HELM, or CHART
}

// isHelmChartFormat returns true if format is a Helm chart variant (HELM_CHART, HELM, or CHART).
func isHelmChartFormat(format string) bool {
	switch strings.ToUpper(format) {
	case "HELM_CHART", "HELM", "CHART":
		return true
	default:
		return false
	}
}

// FetchApplications fetches all applications from the Application Collection Metadata Service
// (GET /applications) with pagination. Uses official API parameters: page_number, page_size,
// and optional packaging_formats=CONTAINER,HELM_CHART to exclude RPM. Returns OCI chart refs
// (oci://dp.apps.rancher.io/charts/<slug>) and container image refs
// (dp.apps.rancher.io/containers/<slug>:latest). user and password are for HTTP Basic Auth;
// if empty, the request is sent without auth.
func FetchApplications(ctx context.Context, user, password string) (chartRefs []string, imageRefs []string, err error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}

	seenCharts := make(map[string]bool)
	seenImages := make(map[string]bool)
	pageNumber := 1
	pageSize := DefaultPageSize
	// Only request CONTAINER and HELM_CHART (exclude RPM). Response may still use HELM/CHART; we treat those as charts.
	packagingFormats := "CONTAINER,HELM_CHART"

	for {
		url := fmt.Sprintf("%s/applications?page_number=%d&page_size=%d&packaging_formats=%s",
			AppsAPIBase, pageNumber, pageSize, packagingFormats)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("appcollection api: %w", err)
		}
		if user != "" || password != "" {
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+password)))
		}

		resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
		if err != nil {
			return nil, nil, fmt.Errorf("appcollection api request: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, nil, fmt.Errorf("appcollection api: unexpected status %d", resp.StatusCode)
		}

		var body ApplicationsResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			resp.Body.Close()
			return nil, nil, fmt.Errorf("appcollection api decode: %w", err)
		}
		resp.Body.Close()

		if len(body.Items) == 0 {
			break
		}

		for _, item := range body.Items {
			slug := strings.TrimSpace(item.SlugName)
			if slug == "" {
				continue
			}
			format := strings.TrimSpace(item.PackagingFormat)
			if isHelmChartFormat(format) {
				ref := fmt.Sprintf("oci://%s/charts/%s", ChartsRegistry, slug)
				if !seenCharts[ref] {
					seenCharts[ref] = true
					chartRefs = append(chartRefs, ref)
				}
			} else if strings.EqualFold(format, PackagingFormatContainer) {
				ref := fmt.Sprintf("%s/%s:latest", ContainersRegistry, slug)
				if !seenImages[ref] {
					seenImages[ref] = true
					imageRefs = append(imageRefs, ref)
				}
			}
			// RPM and any other format are skipped
		}

		logrus.Debugf("Application Collection API page %d/%d: %d items", pageNumber, body.TotalPages, len(body.Items))
		if pageNumber >= body.TotalPages || len(body.Items) < pageSize {
			break
		}
		pageNumber++
	}

	logrus.Infof("Application Collection: fetched %d chart refs, %d container image refs", len(chartRefs), len(imageRefs))
	return chartRefs, imageRefs, nil
}
