package commands

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cnrancher/hangar/pkg/rancher/kdmimages"
	"github.com/cnrancher/hangar/pkg/rancher/listgenerator"
	"github.com/cnrancher/hangar/pkg/utils"
)

// treeNode is one row in the tree: preset, group, chart folder, chart, or image (display-only).
type treeNode struct {
	Id       string
	Label    string
	Kind     string // preset, component, chart_all, chart, image
	Count    int
	Children []treeNode
}

// treeRow is a flattened row for display (with depth for indent).
type treeRow struct {
	Depth  int
	Node   treeNode
	RowIdx int // index in the full flattened list (for selection key)
}

// treeModel shows only 2 groups: Basic and AddOns
type treeModel struct {
	roots          []treeNode
	expanded       map[string]bool // node id -> expanded
	visible        []treeRow       // flattened visible rows (rebuilt when expanded changes)
	cursor         int
	selected       map[string]bool // node id -> selected (for preset/component/chart)
	done           bool
	aborted        bool // true when user pressed q or Ctrl+C to exit immediately
	cniForStandard string
	width          int
	height         int
	// Store chart groups for Basic preview (all charts with images in Basic)
	basicCharts []treeNode
	fleetCharts []treeNode // Kept for backward compatibility
	cniCharts   []treeNode // Kept for backward compatibility
	// basicImageComponent: image ref -> component label for colored legend (Rancher, Fleet, CNI, K3s, RKE2, LB)
	basicImageComponent map[string]string
	// pastSelection: summary of Step 1 (source) + Step 2 (distro, CNI, LB, versions) for footer
	pastSelection string
}

func (m *treeModel) buildVisible() {
	m.visible = nil
	var walk func(nodes []treeNode, depth int)
	walk = func(nodes []treeNode, depth int) {
		for _, n := range nodes {
			m.visible = append(m.visible, treeRow{Depth: depth, Node: n})
			if (n.Kind == "preset" || n.Kind == "component" || n.Kind == "chart_all" || n.Kind == "chart") && m.expanded[n.Id] && len(n.Children) > 0 {
				walk(n.Children, depth+1)
			}
		}
	}
	walk(m.roots, 0)
}

func (m *treeModel) rowAt(i int) (treeRow, bool) {
	if i < 0 || i >= len(m.visible) {
		return treeRow{}, false
	}
	return m.visible[i], true
}

func (m *treeModel) selectable(r treeRow) bool {
	// All nodes are selectable, including images (user can deselect individual images)
	return true
}

func (m *treeModel) findParentId(r treeRow) string {
	// Find parent by walking up the tree
	var findParent func(nodes []treeNode, targetId string, parentId string) string
	findParent = func(nodes []treeNode, targetId string, parentId string) string {
		for _, n := range nodes {
			if n.Id == targetId {
				return parentId
			}
			if found := findParent(n.Children, targetId, n.Id); found != "" {
				return found
			}
		}
		return ""
	}
	return findParent(m.roots, r.Node.Id, "")
}

func (m *treeModel) checkAllChildrenDeselected(n treeNode, parentId string) bool {
	if n.Id == parentId {
		for _, child := range n.Children {
			if m.selected[child.Id] {
				return false
			}
		}
		return true
	}
	for _, child := range n.Children {
		if !m.checkAllChildrenDeselected(child, parentId) {
			return false
		}
	}
	return true
}

func (m *treeModel) Init() tea.Cmd {
	return nil
}

func (m *treeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	mm := m
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		mm.width = msg.Width
		mm.height = msg.Height
		return mm, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			mm.aborted = true
			return mm, tea.Quit
		case "up", "k":
			if mm.cursor > 0 {
				mm.cursor--
			}
			return mm, nil
		case "down", "j":
			if mm.cursor < len(mm.visible)-1 {
				mm.cursor++
			}
			return mm, nil
		case " ":
			if r, ok := mm.rowAt(mm.cursor); ok && mm.selectable(r) {
				newState := !mm.selected[r.Node.Id]
				mm.selected[r.Node.Id] = newState
				if r.Node.Kind == "image" {
					// Image-level toggle; no children to update
				} else {
					// Group/chart: hierarchical selection
					if newState && len(r.Node.Children) > 0 {
						var selectChildren func(n treeNode)
						selectChildren = func(n treeNode) {
							mm.selected[n.Id] = true
							for _, child := range n.Children {
								selectChildren(child)
							}
						}
						for _, child := range r.Node.Children {
							selectChildren(child)
						}
					}
					if !newState && len(r.Node.Children) > 0 {
						var deselectChildren func(n treeNode)
						deselectChildren = func(n treeNode) {
							mm.selected[n.Id] = false
							for _, child := range n.Children {
								deselectChildren(child)
							}
						}
						for _, child := range r.Node.Children {
							deselectChildren(child)
						}
					}
					// If deselecting a child, check if parent should be deselected
					if !newState && r.Depth > 0 {
						parentId := mm.findParentId(r)
						if parentId != "" {
							allDeselected := true
							for _, root := range mm.roots {
								if mm.checkAllChildrenDeselected(root, parentId) {
									allDeselected = true
									break
								}
							}
							if allDeselected {
								mm.selected[parentId] = false
							}
						}
					}
				}
			}
			return mm, nil
		case "right", "l":
			r, ok := mm.rowAt(mm.cursor)
			if !ok {
				return mm, nil
			}
			expandable := (r.Node.Kind == "preset" || r.Node.Kind == "component" || r.Node.Kind == "chart_all" || r.Node.Kind == "chart") && len(r.Node.Children) > 0
			if expandable {
				mm.expanded[r.Node.Id] = !mm.expanded[r.Node.Id]
				mm.buildVisible()
				if mm.cursor >= len(mm.visible) {
					mm.cursor = len(mm.visible) - 1
				}
				return mm, nil
			}
			return mm, nil
		case "enter":
			r, ok := mm.rowAt(mm.cursor)
			if !ok {
				return mm, nil
			}
			expandable := (r.Node.Kind == "preset" || r.Node.Kind == "component" || r.Node.Kind == "chart_all" || r.Node.Kind == "chart") && len(r.Node.Children) > 0
			if expandable {
				mm.expanded[r.Node.Id] = !mm.expanded[r.Node.Id]
				mm.buildVisible()
				if mm.cursor >= len(mm.visible) {
					mm.cursor = len(mm.visible) - 1
				}
				return mm, nil
			}
			return mm, nil
		case "d":
			mm.done = true
			return mm, tea.Quit
		case "left", "h":
			r, ok := mm.rowAt(mm.cursor)
			if !ok {
				return mm, nil
			}
			if r.Depth > 0 && (r.Node.Kind == "preset" || r.Node.Kind == "component" || r.Node.Kind == "chart_all" || r.Node.Kind == "chart") {
				mm.expanded[r.Node.Id] = false
				mm.buildVisible()
				if mm.cursor >= len(mm.visible) {
					mm.cursor = len(mm.visible) - 1
				}
			}
			return mm, nil
		case "tab":
			// Move to next expandable node in the tree
			expandable := func(idx int) bool {
				if idx < 0 || idx >= len(mm.visible) {
					return false
				}
				row := mm.visible[idx]
				return (row.Node.Kind == "preset" || row.Node.Kind == "component" || row.Node.Kind == "chart_all" || row.Node.Kind == "chart") && len(row.Node.Children) > 0
			}
			for i := mm.cursor + 1; i < len(mm.visible); i++ {
				if expandable(i) {
					mm.cursor = i
					return mm, nil
				}
			}
			return mm, nil
		case "shift+tab":
			// Move to previous expandable node
			expandable := func(idx int) bool {
				if idx < 0 || idx >= len(mm.visible) {
					return false
				}
				row := mm.visible[idx]
				return (row.Node.Kind == "preset" || row.Node.Kind == "component" || row.Node.Kind == "chart_all" || row.Node.Kind == "chart") && len(row.Node.Children) > 0
			}
			for i := mm.cursor - 1; i >= 0; i-- {
				if expandable(i) {
					mm.cursor = i
					return mm, nil
				}
			}
			return mm, nil
		}
	}
	return m, nil
}

// collectSelectedItems returns a list of selected charts and their images for preview
func (m *treeModel) collectSelectedItems() (charts []string, images []string) {
	selectedCharts := make(map[string]bool)
	deselectedCharts := make(map[string]bool)

	// Helper to collect charts from a group node
	var collectChartsFromGroup func(n treeNode)
	collectChartsFromGroup = func(n treeNode) {
		if n.Kind == "chart" {
			selectedCharts[n.Id] = true
			// Collect images from this chart
			for _, child := range n.Children {
				if child.Kind == "image" {
					images = append(images, child.Label)
				}
			}
		}
		for _, child := range n.Children {
			collectChartsFromGroup(child)
		}
	}

	// First pass: collect from Basic and AddOns groups
	for _, root := range m.roots {
		if !m.selected[root.Id] {
			continue
		}
		switch root.Kind {
		case "component":
			// Basic group: includes distro, CNI, Rancher components, Fleet
			if root.Id == "basic" {
				// Basic contains all images directly (flat structure)
				// Collect all images from Basic
				for _, child := range root.Children {
					if child.Kind == "image" {
						images = append(images, child.Label)
					}
				}
			}
			// AddOns group: collect charts from all subdirs
			if root.Id == "addons" {
				collectChartsFromGroup(root)
			}
			// Individual component selection
			if root.Id != "basic" && root.Id != "addons" {
				if root.Id == "cni" || root.Id == "fleet" ||
					strings.HasPrefix(root.Id, "addon_") {
					collectChartsFromGroup(root)
				}
			}
		case "chart":
			selectedCharts[root.Id] = true
			for _, child := range root.Children {
				if child.Kind == "image" {
					images = append(images, child.Label)
				}
			}
		}
	}

	// Second pass: find explicitly deselected charts (child of selected parent)
	for _, r := range m.visible {
		if r.Node.Kind == "chart" && !m.selected[r.Node.Id] {
			// Check if parent is selected
			parentId := m.findParentId(r)
			if parentId != "" && m.selected[parentId] {
				deselectedCharts[r.Node.Id] = true
			}
		}
	}

	// Final chart list: selected minus deselected
	for chart := range selectedCharts {
		if !deselectedCharts[chart] {
			charts = append(charts, chart)
		}
	}
	sort.Strings(charts)
	sort.Strings(images)

	return charts, images
}

// getChartsForSelectedGroup returns charts and images for ALL selected groups/charts
// imageSourceGroup value: "addons" or "app_collection" for legend tagging
func (m *treeModel) getChartsForSelectedGroup() (charts []string, images []string, imageSourceGroup map[string]string) {
	seenImages := make(map[string]bool)
	seenCharts := make(map[string]bool)
	imageSourceGroup = make(map[string]string)

	// Helper to recursively collect charts and images from a node (respects per-image selection)
	// sourceTag is "addons" or "app_collection" when collecting from those groups
	var collectChartsAndImages func(n treeNode, includeChildren bool, sourceTag string)
	collectChartsAndImages = func(n treeNode, includeChildren bool, sourceTag string) {
		if n.Kind == "chart" {
			if !seenCharts[n.Label] {
				charts = append(charts, n.Label)
				seenCharts[n.Label] = true
			}
			for _, child := range n.Children {
				if child.Kind == "image" && m.selected[child.Id] && !seenImages[child.Label] {
					images = append(images, child.Label)
					seenImages[child.Label] = true
					if sourceTag != "" {
						imageSourceGroup[child.Label] = sourceTag
					}
				}
			}
		}
		if includeChildren {
			for _, child := range n.Children {
				collectChartsAndImages(child, true, sourceTag)
			}
		}
	}

	// Collect from ALL selected items in the visible tree
	// Preview shows union of: Basic (if selected) + AddOns/subgroups/charts (if selected)
	for _, r := range m.visible {
		if !m.selected[r.Node.Id] {
			continue
		}

		// Basic group: only collect from selected subgroups and selected images
		if r.Node.Id == "basic" {
			var collectBasicImages func(n treeNode)
			collectBasicImages = func(n treeNode) {
				if n.Kind == "image" && m.selected[n.Id] && !seenImages[n.Label] {
					images = append(images, n.Label)
					seenImages[n.Label] = true
				}
				for _, child := range n.Children {
					collectBasicImages(child)
				}
			}
			basicIncludedImages := make(map[string]bool)
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					collectBasicImages(child)
					for _, imgNode := range child.Children {
						if imgNode.Kind == "image" && m.selected[imgNode.Id] {
							basicIncludedImages[imgNode.Label] = true
						}
					}
				}
			}
			// If no subgroup selected, Basic means all subgroups (backward compat)
			if len(basicIncludedImages) == 0 {
				for _, child := range r.Node.Children {
					if m.selected[child.Id] {
						collectBasicImages(child)
					}
				}
				for _, chart := range m.basicCharts {
					if !seenCharts[chart.Label] {
						charts = append(charts, chart.Label)
						seenCharts[chart.Label] = true
					}
				}
			} else {
				// Only include basic charts that have at least one image in the selected Basic set
				for _, chart := range m.basicCharts {
					hasIncluded := false
					for _, ch := range chart.Children {
						if ch.Kind == "image" && basicIncludedImages[ch.Label] {
							hasIncluded = true
							break
						}
					}
					if hasIncluded && !seenCharts[chart.Label] {
						charts = append(charts, chart.Label)
						seenCharts[chart.Label] = true
					}
				}
			}
			continue
		}

		// Application Collection: Charts + Containers (same structure as Essentials/AddOns)
		if r.Node.Id == "app_collection" {
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					collectChartsAndImages(child, true, "app_collection")
				}
			}
			continue
		}
		if r.Node.Id == "app_collection_charts" {
			hasSelectedCharts := false
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					hasSelectedCharts = true
					break
				}
			}
			if hasSelectedCharts {
				for _, child := range r.Node.Children {
					if m.selected[child.Id] {
						collectChartsAndImages(child, true, "app_collection")
					}
				}
			} else {
				collectChartsAndImages(r.Node, true, "app_collection")
			}
			continue
		}
		if r.Node.Id == listgenerator.SourceGroupAppCollectionContainers {
			for _, child := range r.Node.Children {
				if child.Kind == "image" && m.selected[child.Id] && !seenImages[child.Label] {
					images = append(images, child.Label)
					seenImages[child.Label] = true
					imageSourceGroup[child.Label] = "app_collection"
				}
			}
			continue
		}

		// Essentials subgroups (e.g. CNI, Rancher, K3s): when Essentials is deselected but subgroup is selected
		if strings.HasPrefix(r.Node.Id, "basic_") {
			for _, child := range r.Node.Children {
				if child.Kind == "image" && m.selected[child.Id] && !seenImages[child.Label] {
					images = append(images, child.Label)
					seenImages[child.Label] = true
				}
			}
			// Include basic charts that have at least one image in this subgroup
			subgroupImages := make(map[string]bool)
			for _, child := range r.Node.Children {
				if child.Kind == "image" {
					subgroupImages[child.Label] = true
				}
			}
			for _, chart := range m.basicCharts {
				if seenCharts[chart.Label] {
					continue
				}
				for _, ch := range chart.Children {
					if ch.Kind == "image" && subgroupImages[ch.Label] {
						charts = append(charts, chart.Label)
						seenCharts[chart.Label] = true
						break
					}
				}
			}
			continue
		}

		// AddOns group: collect from all selected subgroups/charts (always process when selected)
		if r.Node.Id == "addons" {
			hasSelectedSubgroups := false
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					hasSelectedSubgroups = true
					break
				}
			}
			if hasSelectedSubgroups {
				for _, child := range r.Node.Children {
					if m.selected[child.Id] {
						collectChartsAndImages(child, true, "addons")
					}
				}
			} else {
				collectChartsAndImages(r.Node, true, "addons")
			}
			continue
		}

		// Subgroups (like Monitoring, Logging, etc.): collect charts and images
		if strings.HasPrefix(r.Node.Id, "addon_") {
			hasSelectedCharts := false
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					hasSelectedCharts = true
					break
				}
			}
			if hasSelectedCharts {
				for _, child := range r.Node.Children {
					if m.selected[child.Id] {
						collectChartsAndImages(child, true, "addons")
					}
				}
			} else {
				collectChartsAndImages(r.Node, true, "addons")
			}
			continue
		}

		// Individual chart: collect images from this chart (no group tag)
		if r.Node.Kind == "chart" {
			collectChartsAndImages(r.Node, false, "")
		}
	}

	// Remove duplicates
	chartMap := make(map[string]bool)
	var uniqueCharts []string
	for _, chart := range charts {
		if !chartMap[chart] {
			chartMap[chart] = true
			uniqueCharts = append(uniqueCharts, chart)
		}
	}

	imageMap := make(map[string]bool)
	var uniqueImages []string
	for _, img := range images {
		if !imageMap[img] {
			imageMap[img] = true
			uniqueImages = append(uniqueImages, img)
		}
	}

	sort.Strings(uniqueCharts)
	sort.Strings(uniqueImages)
	return uniqueCharts, uniqueImages, imageSourceGroup
}

// collectSelectedImageRefs returns the exact set of image refs that are selected in the tree.
// Respects per-image selection (user can deselect individual images).
func (m *treeModel) collectSelectedImageRefs() []string {
	seen := make(map[string]bool)
	var images []string

	var collectImages func(n treeNode)
	collectImages = func(n treeNode) {
		if n.Kind == "image" && m.selected[n.Id] && !seen[n.Label] {
			images = append(images, n.Label)
			seen[n.Label] = true
		}
		for _, child := range n.Children {
			collectImages(child)
		}
	}

	for _, r := range m.visible {
		if !m.selected[r.Node.Id] {
			continue
		}
		if r.Node.Id == "basic" {
			// Only collect images from selected basic_* subgroups (user can deselect Fleet, CNI, etc.)
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					collectImages(child)
				}
			}
			continue
		}
		// basic_* subgroup selected: collect only that subgroup's images
		if strings.HasPrefix(r.Node.Id, "basic_") {
			collectImages(r.Node)
			continue
		}
		if r.Node.Id == "app_collection" {
			// Collect from selected children only (Charts and/or Containers, same as Essentials/AddOns)
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					collectImages(child)
				}
			}
			continue
		}
		if r.Node.Id == "app_collection_charts" {
			// Application Collection → Charts: collect from selected charts or all
			hasSelectedCharts := false
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					hasSelectedCharts = true
					break
				}
			}
			if hasSelectedCharts {
				for _, child := range r.Node.Children {
					if m.selected[child.Id] {
						collectImages(child)
					}
				}
			} else {
				collectImages(r.Node)
			}
			continue
		}
		if r.Node.Id == listgenerator.SourceGroupAppCollectionContainers {
			collectImages(r.Node)
			continue
		}
		if r.Node.Id == "addons" {
			hasSelectedSubgroups := false
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					hasSelectedSubgroups = true
					break
				}
			}
			if hasSelectedSubgroups {
				for _, child := range r.Node.Children {
					if m.selected[child.Id] {
						collectImages(child)
					}
				}
			} else {
				collectImages(r.Node)
			}
			continue
		}
		if strings.HasPrefix(r.Node.Id, "addon_") {
			hasSelectedCharts := false
			for _, child := range r.Node.Children {
				if m.selected[child.Id] {
					hasSelectedCharts = true
					break
				}
			}
			if hasSelectedCharts {
				for _, child := range r.Node.Children {
					if m.selected[child.Id] {
						collectImages(child)
					}
				}
			} else {
				collectImages(r.Node)
			}
			continue
		}
		if r.Node.Kind == "chart" {
			collectImages(r.Node)
		}
	}
	return images
}

func (m *treeModel) View() string {
	// Default width if not set
	width := m.width
	if width == 0 {
		width = 160
	}

	// Split into 3 columns: 30% groups, 30% charts, 40% images
	col1Width := int(float64(width) * 0.3)
	col2Width := int(float64(width) * 0.3)
	col3Width := width - col1Width - col2Width - 2 // -2 for separators

	// Build column 1 (groups/tree)
	var col1Builder strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render
	col1Builder.WriteString(title("Step 3: Groups") + "\n")
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	col1Builder.WriteString(descStyle.Render("Essentials = Rancher core (incl. Fleet), CNI, distro, LB. AddOns = monitoring, logging, storage, etc.") + "\n")
	col1Builder.WriteString(descStyle.Render("Same options via: --config <file> (see generate-list-config.example.yaml)") + "\n\n")
	col1Builder.WriteString("↑/↓ move   Space toggle (groups, charts, images)   ←/→ expand   Tab next   d done\n\n")

	for i, r := range m.visible {
		indent := strings.Repeat("  ", r.Depth)
		prefix := "  "
		if m.selectable(r) && m.selected[r.Node.Id] {
			prefix = "X "
		}
		line := indent + prefix + r.Node.Label
		if r.Node.Count > 0 && r.Node.Kind != "image" {
			line += fmt.Sprintf(" (%d)", r.Node.Count)
		}
		expandable := (r.Node.Kind == "preset" || r.Node.Kind == "component" || r.Node.Kind == "chart_all" || r.Node.Kind == "chart") && len(r.Node.Children) > 0
		if expandable {
			if m.expanded[r.Node.Id] {
				line += " ▼"
			} else {
				line += " ▶"
			}
		}
		if i == m.cursor {
			// Preserve the selection symbol when cursor is on the line
			line = "▸ " + lipgloss.NewStyle().Bold(true).Render(line)
		} else {
			// Remove leading spaces but keep selection symbol
			if strings.HasPrefix(line, "  ") {
				line = line[2:]
			}
		}
		// Truncate if too long
		if len(line) > col1Width-2 {
			line = line[:col1Width-5] + "..."
		}
		col1Builder.WriteString(line + "\n")
	}

	// Build column 2 (charts for selected group)
	var col2Builder strings.Builder
	col2Title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render
	col2Builder.WriteString(col2Title("Charts") + "\n")
	col2Builder.WriteString(strings.Repeat("─", col2Width-2) + "\n\n")

	charts, images, imageSourceGroup := m.getChartsForSelectedGroup()

	if len(charts) > 0 {
		maxCharts := 100
		displayCharts := charts
		if len(displayCharts) > maxCharts {
			displayCharts = displayCharts[:maxCharts]
		}
		for _, chart := range displayCharts {
			chartName := chart
			if len(chartName) > col2Width-4 {
				chartName = chartName[:col2Width-7] + "..."
			}
			col2Builder.WriteString("  • " + chartName + "\n")
		}
		if len(charts) > maxCharts {
			col2Builder.WriteString(fmt.Sprintf("\n  ... and %d more\n", len(charts)-maxCharts))
		}
	} else {
		col2Builder.WriteString("Select a group to see charts.\n")
	}

	// Build column 3 (images for selected charts)
	var col3Builder strings.Builder
	col3Title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render
	col3Builder.WriteString(col3Title(fmt.Sprintf("Images (%d)", len(images))) + "\n")
	col3Builder.WriteString(strings.Repeat("─", col3Width-2) + "\n\n")

	// Show legend: Essentials tags when basic selected; always show AddOns and Application Collection
	basicSelected := false
	addonsSelected := false
	appCollSelected := false
	for _, r := range m.visible {
		if r.Node.Id == "basic" && m.selected[r.Node.Id] {
			basicSelected = true
		}
		if r.Node.Id == "addons" && m.selected[r.Node.Id] {
			addonsSelected = true
		}
		if r.Node.Id == "app_collection" && m.selected[r.Node.Id] {
			appCollSelected = true
		}
	}
	legendStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(false)
	showLegend := basicSelected || addonsSelected || appCollSelected
	if showLegend {
		col3Builder.WriteString(legendStyle.Render("Legend: "))
		if basicSelected && len(m.basicImageComponent) > 0 {
			col3Builder.WriteString(
				lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("R") + "=Rancher " +
					lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("C") + "=CNI " +
					lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Render("D") + "=Distro " +
					lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("LB") + "=Load balancer")
		}
		if addonsSelected || appCollSelected {
			if basicSelected && len(m.basicImageComponent) > 0 {
				col3Builder.WriteString("  ")
			}
			if addonsSelected {
				col3Builder.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("Ad") + "=AddOns ")
			}
			if appCollSelected {
				col3Builder.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("Ac") + "=Application Collection ")
			}
		}
		col3Builder.WriteString("\n\n")
	}

	if len(images) > 0 {
		maxImages := 200
		displayImages := images
		if len(displayImages) > maxImages {
			displayImages = displayImages[:maxImages]
		}
		tagColor := map[string]lipgloss.Color{
			"Rancher": "12", "Fleet": "12", "CNI": "11", "K3s": "13", "RKE2": "13", "RKE1": "13", "Distro": "13",
			"LB":     "14",
			"addons": "6", "app_collection": "5",
		}
		for _, img := range displayImages {
			imgName := img
			if len(imgName) > col3Width-4 {
				imgName = imgName[:col3Width-7] + "..."
			}
			prefix := "  • "
			if basicSelected && m.basicImageComponent != nil {
				if comp := m.basicImageComponent[img]; comp != "" {
					short := "?"
					if comp == "LB" {
						short = "LB"
					} else if comp == "Rancher" || comp == "Fleet" {
						short = "R"
					} else if comp == "CNI" {
						short = "C"
					} else if comp == "K3s" || comp == "RKE2" || comp == "RKE1" || comp == "Distro" {
						short = "D"
					}
					c := tagColor[comp]
					if c == "" {
						c = "8"
					}
					prefix = "  " + lipgloss.NewStyle().Foreground(c).Bold(true).Render("["+short+"] ") + " "
				}
			}
			if prefix == "  • " && imageSourceGroup != nil {
				if src := imageSourceGroup[img]; src == "addons" {
					c := tagColor["addons"]
					prefix = "  " + lipgloss.NewStyle().Foreground(c).Bold(true).Render("[Ad] ") + " "
				} else if src == "app_collection" {
					c := tagColor["app_collection"]
					prefix = "  " + lipgloss.NewStyle().Foreground(c).Bold(true).Render("[Ac] ") + " "
				}
			}
			col3Builder.WriteString(prefix + imgName + "\n")
		}
		if len(images) > maxImages {
			col3Builder.WriteString(fmt.Sprintf("\n  ... and %d more\n", len(images)-maxImages))
		}
	} else {
		col3Builder.WriteString("Select a group to see images.\n")
	}

	// Style the columns; reserve 2 lines at bottom for selection footer so it's always visible
	col1Style := lipgloss.NewStyle().Width(col1Width)
	col2Style := lipgloss.NewStyle().Width(col2Width).Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("8"))
	col3Style := lipgloss.NewStyle().Width(col3Width).Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("8"))
	contentHeight := m.height
	if m.height > 2 {
		contentHeight = m.height - 2
	}
	if contentHeight > 0 {
		col1Style = col1Style.MaxHeight(contentHeight)
		col2Style = col2Style.MaxHeight(contentHeight)
		col3Style = col3Style.MaxHeight(contentHeight)
	}

	col1Content := col1Style.Render(col1Builder.String())
	col2Content := col2Style.Render(col2Builder.String())
	col3Content := col3Style.Render(col3Builder.String())

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, col1Content, col2Content, col3Content)
	// Selection footer: past steps (Step 1 + Step 2) then current (Step 3 groups/charts)
	var selParts []string
	for _, r := range m.visible {
		if r.Node.Kind != "image" && m.selected[r.Node.Id] {
			label := r.Node.Label
			if idx := strings.Index(label, " ("); idx > 0 {
				label = label[:idx]
			}
			selParts = append(selParts, label)
		}
	}
	sel := "(none)"
	if len(selParts) > 0 {
		sel = strings.Join(selParts, ", ")
		if width > 0 && len(sel) > width-14 {
			sel = sel[:width-17] + "..."
		}
	}
	footerLine := "Selection: " + sel
	if m.pastSelection != "" {
		footerLine = "Selection: " + m.pastSelection + "  →  " + sel
		if width > 0 && len(footerLine) > width-2 {
			maxPast := width - len("Selection: ") - len("  →  ") - 24
			if maxPast > 0 && len(m.pastSelection) > maxPast {
				footerLine = "Selection: " + m.pastSelection[:maxPast-3] + "...  →  " + sel
			}
			if len(footerLine) > width-2 {
				maxCur := width - len("Selection: ") - len("  →  ") - 20
				if maxCur > 0 && len(sel) > maxCur {
					sel = sel[:maxCur-3] + "..."
					footerLine = "Selection: " + m.pastSelection + "  →  " + sel
				}
			}
		}
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render(footerLine)
	return lipgloss.JoinVertical(lipgloss.Left, mainContent, footer)
}

// runTreeTUI runs the tree TUI (Step 3: Groups).
// components should be a comma-separated string of selected cluster types from Step 2 (e.g., "k3s,rke2").
// pastSelection is the summary of Step 1 (source) + Step 2 (distro, CNI, LB, versions) to show in the footer.
func runTreeTUI(roots []treeNode, cniForStandard string, components string, basicCharts []treeNode, fleetCharts []treeNode, cniCharts []treeNode, basicImageComponent map[string]string, pastSelection string) (componentIDs []string, chartNames []string, selectedImageRefs []string, err error) {
	expanded := make(map[string]bool)
	selected := make(map[string]bool)
	// Pre-select Basic so core (Rancher + distro + CNI + Fleet) is included by default
	selected["basic"] = true
	m := &treeModel{
		roots:               roots,
		expanded:            expanded,
		cursor:              0,
		selected:            selected,
		cniForStandard:      cniForStandard,
		basicCharts:         basicCharts,
		fleetCharts:         fleetCharts,
		cniCharts:           cniCharts,
		basicImageComponent: basicImageComponent,
		pastSelection:       pastSelection,
	}
	m.buildVisible()
	// Initialize all image nodes as selected so user can deselect individual images
	var initImageSelection func(nodes []treeNode)
	initImageSelection = func(nodes []treeNode) {
		for _, n := range nodes {
			if n.Kind == "image" {
				selected[n.Id] = true
			}
			initImageSelection(n.Children)
		}
	}
	initImageSelection(roots)
	// Pre-select all Basic subgroups so they show as selected when Basic is selected
	for _, root := range roots {
		if root.Id == "basic" {
			for _, child := range root.Children {
				selected[child.Id] = true
			}
			break
		}
	}

	// Don't use alt screen so logs remain visible
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, nil, nil, err
	}
	mm := final.(*treeModel)
	if mm.aborted {
		return nil, nil, nil, ErrAborted
	}

	// Collect all selected chart names (including from selected groups)
	selectedCharts := make(map[string]bool)
	deselectedCharts := make(map[string]bool)

	// Helper to collect charts from a group node
	var collectChartsFromGroup func(n treeNode)
	collectChartsFromGroup = func(n treeNode) {
		if n.Kind == "chart" {
			selectedCharts[n.Id] = true
		}
		for _, child := range n.Children {
			collectChartsFromGroup(child)
		}
	}

	// Map Essentials subgroup ID (basic_*) to listgenerator component IDs
	basicSubgroupToComponentIDs := func(basicID string) []string {
		switch basicID {
		case "basic_rancher":
			return []string{"system_addons", "fleet"}
		case "basic_cni":
			if mm.cniForStandard != "" {
				return []string{mm.cniForStandard}
			}
			return []string{"cni"}
		case "basic_k3s":
			return []string{listgenerator.SourceGroupK3s}
		case "basic_rke2":
			return []string{listgenerator.SourceGroupRKE2}
		case "basic_rke1":
			return []string{listgenerator.SourceGroupRKE1}
		case "basic_distro":
			return []string{listgenerator.SourceGroupK3s, listgenerator.SourceGroupRKE2, listgenerator.SourceGroupRKE1}
		case "basic_lb":
			// LB images come from K3s/RKE2; no separate component ID, images collected via selectedImageRefs
			return nil
		default:
			return nil
		}
	}

	// First pass: collect from Basic and AddOns groups
	for _, r := range mm.visible {
		if !mm.selected[r.Node.Id] || !mm.selectable(r) {
			continue
		}
		switch r.Node.Kind {
		case "component":
			// Basic group: includes distro, CNI, Rancher components, Fleet
			if r.Node.Id == "basic" {
				// Basic contains all images directly, so use BasicPresetWithCNI to get component IDs
				componentIDs = listgenerator.BasicPresetWithCNI(components, mm.cniForStandard)
				componentIDs = append(componentIDs, "fleet")
			}
			// Essentials subgroups (basic_*): when Essentials is deselected but subgroup is selected
			if strings.HasPrefix(r.Node.Id, "basic_") {
				componentIDs = append(componentIDs, basicSubgroupToComponentIDs(r.Node.Id)...)
				// Add basic charts that reference at least one image in this subgroup
				subgroupImages := make(map[string]bool)
				for _, child := range r.Node.Children {
					if child.Kind == "image" {
						subgroupImages[child.Label] = true
					}
				}
				for _, chart := range mm.basicCharts {
					for _, ch := range chart.Children {
						if ch.Kind == "image" && subgroupImages[ch.Label] {
							selectedCharts[chart.Id] = true
							break
						}
					}
				}
			}
			// AddOns group: collect charts from all subdirs
			if r.Node.Id == "addons" {
				collectChartsFromGroup(r.Node)
			}
			// Application Collection: Charts + Containers (same structure as Essentials/AddOns)
			if r.Node.Id == "app_collection" {
				componentIDs = append(componentIDs, listgenerator.SourceGroupAppCollection)
				componentIDs = append(componentIDs, listgenerator.SourceGroupAppCollectionContainers)
				collectChartsFromGroup(r.Node)
			}
			if r.Node.Id == "app_collection_charts" {
				componentIDs = append(componentIDs, listgenerator.SourceGroupAppCollection)
				collectChartsFromGroup(r.Node)
			}
			// Application Collection → Containers (container-only images)
			if r.Node.Id == listgenerator.SourceGroupAppCollectionContainers {
				componentIDs = append(componentIDs, listgenerator.SourceGroupAppCollectionContainers)
			}
			// Individual component selection (for backward compatibility)
			if r.Node.Id != "basic" && r.Node.Id != "addons" && r.Node.Id != "app_collection" && r.Node.Id != "app_collection_charts" && r.Node.Id != listgenerator.SourceGroupAppCollectionContainers && !strings.HasPrefix(r.Node.Id, "basic_") {
				// Addon subgroups (addon_*) should ONLY collect charts, not add component IDs
				// Component IDs are for functional groups that match images by name patterns
				if strings.HasPrefix(r.Node.Id, "addon_") {
					// Addon subgroups: only collect charts, don't add component ID
					collectChartsFromGroup(r.Node)
				} else {
					// Non-addon components (cni, fleet, etc.): add component ID and collect charts
					componentIDs = append(componentIDs, r.Node.Id)
					if r.Node.Id == "cni" || r.Node.Id == "fleet" {
						collectChartsFromGroup(r.Node)
					}
				}
			}
		case "chart":
			selectedCharts[r.Node.Id] = true
		}
	}

	// Second pass: find explicitly deselected charts (child of selected parent)
	for _, r := range mm.visible {
		if r.Node.Kind == "chart" && !mm.selected[r.Node.Id] {
			// Check if parent (group or preset) is selected
			parentId := mm.findParentId(r)
			if parentId != "" {
				// Check if parent is selected (could be group or preset)
				if mm.selected[parentId] {
					deselectedCharts[r.Node.Id] = true
				} else {
					// Check if parent's parent (preset) is selected
					for _, root := range mm.roots {
						if root.Id == parentId && mm.selected[root.Id] {
							// Check if this chart is in preset's children
							var inPreset bool
							var checkPreset func(n treeNode)
							checkPreset = func(n treeNode) {
								if n.Id == r.Node.Id {
									inPreset = true
									return
								}
								for _, child := range n.Children {
									checkPreset(child)
								}
							}
							checkPreset(root)
							if inPreset {
								deselectedCharts[r.Node.Id] = true
							}
						}
					}
				}
			}
		}
	}

	// Final chart list: selected minus deselected
	for chart := range selectedCharts {
		if !deselectedCharts[chart] {
			chartNames = append(chartNames, chart)
		}
	}
	sort.Strings(chartNames)

	// Collect exact image refs from the tree so output matches preview
	selectedImageRefs = mm.collectSelectedImageRefs()
	sort.Strings(selectedImageRefs)

	return componentIDs, chartNames, selectedImageRefs, nil
}

// imagesFromGroup returns sorted image refs from a ComponentGroup.
func imagesFromGroup(g *listgenerator.ComponentGroup) []string {
	if g == nil {
		return nil
	}
	var out []string
	for img := range g.LinuxImages {
		out = append(out, img)
	}
	for img := range g.WindowsImages {
		out = append(out, img)
	}
	sort.Strings(out)
	return out
}

// --- Source type TUI (first question: Community vs Prime GC) ---

type sourceTypeModel struct {
	cursor  int
	done    bool
	aborted bool // true when user pressed q or Ctrl+C
	width   int
	height  int
}

func (m sourceTypeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < 1 {
				m.cursor++
			}
			return m, nil
		case "enter", " ":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m sourceTypeModel) Init() tea.Cmd { return nil }

func (m sourceTypeModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render
	desc := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true).Render
	opts := []string{
		"Community (Rancher Prime Manager – charts from GitHub, KDM from releases.rancher.com)",
		"Rancher Prime (Rancher Prime Registry – charts from charts.rancher.com, KDM from releases.rancher.com)",
	}
	var b strings.Builder
	b.WriteString(title("Step 1: Source – Community or Rancher Prime?") + "\n\n")
	b.WriteString(desc("Community: charts from GitHub (rancher/charts), KDM from releases.rancher.com. Rancher Prime: charts from charts.rancher.com, KDM from releases.rancher.com.") + "\n")
	b.WriteString(desc("Coming: generic Helm and OCI chart integrations into image-list.") + "\n\n")
	b.WriteString("↑/↓ move   Enter confirm   q quit   Ctrl+C exit\n\n")
	for i, opt := range opts {
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		b.WriteString(prefix + opt + "\n")
	}
	b.WriteString("\nPress Enter to confirm.\n")
	sel := "Community"
	if m.cursor == 1 {
		sel = "Rancher Prime"
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render("Selection: " + sel)
	return b.String() + "\n" + footer
}

// RunSourceTypeTUI runs the first TUI step: Community vs Prime GC. Returns isPrimeGC (true = Prime GC).
func RunSourceTypeTUI() (isPrimeGC bool, err error) {
	m := sourceTypeModel{cursor: 0, done: false, aborted: false}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return false, err
	}
	mm := final.(sourceTypeModel)
	if mm.aborted {
		return false, ErrAborted
	}
	isPrimeGC = (mm.cursor == 1)
	return isPrimeGC, nil
}

// --- Application Collection TUI (after source type: include charts from dp.apps.rancher.io?) ---

const (
	AppCollectionRegistry = "dp.apps.rancher.io"
	AppCollectionHelp     = "Requires: helm registry login " + AppCollectionRegistry + " -u <user> -p <pass>"
)

type appCollectionModel struct {
	include bool
	done    bool
	aborted bool // true when user pressed q or Ctrl+C
	width   int
	height  int
}

func (m appCollectionModel) Init() tea.Cmd { return nil }

func (m appCollectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		case "y", "Y", "enter":
			m.include = true
			m.done = true
			return m, tea.Quit
		case "n", "N", "esc":
			m.include = false
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m appCollectionModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render
	desc := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true).Render
	var b strings.Builder
	b.WriteString(title("Include charts from Application Collection?") + "\n\n")
	b.WriteString(desc("Charts from "+AppCollectionRegistry+" (Rancher Application Collection).") + "\n")
	b.WriteString(desc(AppCollectionHelp) + "\n\n")
	b.WriteString(desc("Note: KDM is not provided by Application Collection; it is from releases.rancher.com or Rancher Prime.") + "\n\n")
	b.WriteString("  [y] Yes – live-fetch charts and container images from api.apps.rancher.io\n")
	b.WriteString("  [n] No  – only use chart sources from the previous step (default)\n\n")
	b.WriteString(desc("If Yes, you will be prompted for username and access token (same as curl -u user:token).") + "\n\n")
	b.WriteString("y / n   Enter = Yes   q quit   Ctrl+C exit\n")
	sel := "No"
	if m.include {
		sel = "Yes"
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render("Selection: Include App Collection: " + sel)
	return b.String() + "\n" + footer
}

// RunIncludeAppCollectionTUI runs after source type: ask whether to include charts from Application Collection (dp.apps.rancher.io).
// Returns true if user chose Yes. KDM is not available from this registry.
func RunIncludeAppCollectionTUI() (include bool, err error) {
	m := appCollectionModel{include: false, done: false, aborted: false}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return false, err
	}
	mm := final.(appCollectionModel)
	if mm.aborted {
		return false, ErrAborted
	}
	return mm.include, nil
}

// RunAppCollectionCredentialsTUI prompts for Application Collection API credentials (username and access token)
// used for api.apps.rancher.io. Same credentials as curl -u username:token.
// Returns username, password (token), and error. Call this only when the user selected Application Collection.
func RunAppCollectionCredentialsTUI() (username, password string, err error) {
	fmt.Println()
	fmt.Print("Application Collection API – Username (e.g. your@email.com): ")
	var user string
	if _, err := utils.Scanf(signalContext, "%s\n", &user); err != nil {
		return "", "", fmt.Errorf("read username: %w", err)
	}
	user = strings.TrimSpace(user)
	if user == "" {
		return "", "", fmt.Errorf("username is required for Application Collection API")
	}
	fmt.Print("Application Collection API – Password / Access token: ")
	passBytes, err := utils.ReadPassword(signalContext)
	if err != nil {
		return "", "", fmt.Errorf("read password: %w", err)
	}
	password = strings.TrimSpace(string(passBytes))
	if password == "" {
		return "", "", fmt.Errorf("password/token is required for Application Collection API")
	}
	fmt.Println()
	return user, password, nil
}

// Step1Details is shown in the Details panel during Step 1 (KDM and image list sources).
type Step1Details struct {
	KDMURL          string
	ImageListSource string
}

// Step 1 documentation URLs (Rancher Manager, RKE2, K3s, CNI, ingress).
const (
	docRancherManager = "https://ranchermanager.docs.rancher.com"
	docRancherCNI     = "https://ranchermanager.docs.rancher.com/faq/container-network-interface-providers"
	docRKE2           = "https://docs.rke2.io"
	docRKE2Install    = "https://docs.rke2.io/quick-start"
	docRKE2Windows    = "https://docs.rke2.io/reference/windows_agent_config"
	docRKE2Networking = "https://docs.rke2.io/networking/networking_services"
	docK3s            = "https://docs.k3s.io"
	docK3sNetworking  = "https://docs.k3s.io/networking"
	docK3sInstall     = "https://docs.k3s.io/quick-start"
	docRKE1           = "https://rke.docs.rancher.com"
	docCanal          = "https://projectcalico.docs.tigera.io/getting-started/kubernetes/flannel/flannel"
	docCalico         = "https://docs.tigera.io/calico/latest/about"
	docCilium         = "https://docs.cilium.io"
	docFlannel        = "https://github.com/flannel-io/flannel"
	docTraefik        = "https://doc.traefik.io/traefik"
	docNGINXIngress   = "https://kubernetes.github.io/ingress-nginx"
)

// --- Step 1 TUI: cluster types + CNI ---

type step1Row struct {
	kind  string // "cluster" or "cni"
	id    string
	label string
}

type step1Model struct {
	rows          []step1Row
	cursor        int
	selected      map[int]bool
	done          bool
	wantBack      bool // true when user pressed 'b' to return to previous step
	aborted       bool // true when user pressed q or Ctrl+C to exit immediately
	showRKE1      bool
	width         int
	height        int
	details       Step1Details
	pastSelection string // summary of previous steps (distro, CNI, LB) for footer
}

func (m step1Model) Init() tea.Cmd {
	return nil
}

func (m step1Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	mm := &m
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		mm.width = msg.Width
		mm.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			mm.aborted = true
			return m, tea.Quit
		case "up", "k":
			if mm.cursor > 0 {
				mm.cursor--
			}
			return m, nil
		case "down", "j":
			if mm.cursor < len(mm.rows)-1 {
				mm.cursor++
			}
			return m, nil
		case " ":
			idx := mm.cursor
			if idx >= 0 && idx < len(mm.rows) {
				r := mm.rows[idx]
				if r.kind == "cluster" {
					mm.selected[idx] = !mm.selected[idx]
				}
				if r.kind == "cni" {
					for i := range mm.rows {
						if mm.rows[i].kind == "cni" {
							mm.selected[i] = (i == idx)
						}
					}
				}
				if r.kind == "lb" {
					// Each LB option toggles independently (K3s Klipper, K3s Traefik, RKE2 NGINX, RKE2 Traefik)
					mm.selected[idx] = !mm.selected[idx]
				}
				if r.kind == "platform" {
					// Single choice: Linux only vs Linux + Windows
					for i := range mm.rows {
						if mm.rows[i].kind == "platform" {
							mm.selected[i] = (i == idx)
						}
					}
				}
			}
			return m, nil
		case "enter":
			mm.done = true
			return m, tea.Quit
		case "b", "B":
			// Return to step above (previous screen)
			mm.wantBack = true
			mm.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m step1Model) View() string {
	width := m.width
	if width == 0 {
		width = 120 // Default width
	}
	leftWidth := int(float64(width) * 0.5)
	rightWidth := width - leftWidth - 1

	// Build left column (selection)
	var leftBuilder strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render

	// Determine stage based on row types (distro stage: first row is cluster)
	isDistroStage := len(m.rows) > 0 && m.rows[0].kind == "cluster"
	isCNIStage := len(m.rows) > 0 && m.rows[0].kind == "cni"
	isLBStage := len(m.rows) > 0 && m.rows[0].kind == "lb"

	var stageTitle string
	if isDistroStage {
		stageTitle = "Step 2.1: Select Distro"
	} else if isCNIStage {
		stageTitle = "Step 2.2: Select CNI"
	} else if isLBStage {
		stageTitle = "Step 2.3: Load balancer / Ingress"
	} else {
		stageTitle = "Step 2: Cluster & versions"
	}

	leftBuilder.WriteString(title(stageTitle) + "\n")
	// Stage-specific description (visible in TUI)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	if isDistroStage {
		leftBuilder.WriteString(descStyle.Render("Select Kubernetes distros and platform: Linux only or Linux + Windows (RKE2/K3s Windows node images).") + "\n\n")
	} else if isCNIStage {
		leftBuilder.WriteString(descStyle.Render("Cluster networking (CNI). Flannel is only available for K3s.") + "\n\n")
		leftBuilder.WriteString("CNI:\n")
	} else if isLBStage {
		leftBuilder.WriteString(descStyle.Render("Include load balancer/ingress in Basic? (K3s: Klipper/Traefik, RKE2: NGINX/Traefik)") + "\n\n")
	}
	backHint := ""
	if !isDistroStage {
		backHint = "   b back to step above"
	}
	leftBuilder.WriteString("↑/↓ move   Space toggle   Enter confirm" + backHint + "   q quit   Ctrl+C exit\n\n")

	for i, r := range m.rows {
		prefix := "  "
		if m.selected[i] {
			prefix = "X "
		}
		line := prefix + r.label
		if i == m.cursor {
			// Preserve the selection symbol when cursor is on the line
			line = "▸ " + lipgloss.NewStyle().Bold(true).Render(line)
		} else {
			// Remove leading spaces but keep selection symbol
			if strings.HasPrefix(line, "  ") {
				line = line[2:]
			}
		}
		if isCNIStage {
			line = "  " + line // Extra indent for CNI sub-items
		}
		if isLBStage {
			line = "  " + line // Extra indent for LB options
		}
		leftBuilder.WriteString(line + "\n")
	}
	if isDistroStage {
		leftBuilder.WriteString("\nPress Enter when done.\n")
	} else {
		leftBuilder.WriteString("\nPress Enter when done, or b to go back to the previous step.\n")
	}

	// Build right column (Details: KDM + image list source + selection preview)
	var rightBuilder strings.Builder
	rightTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render
	rightBuilder.WriteString(rightTitle("Details") + "\n")
	rightBuilder.WriteString(strings.Repeat("─", rightWidth-2) + "\n\n")

	// KDM and image list source (from config / source-type selection)
	if m.details.KDMURL != "" || m.details.ImageListSource != "" {
		rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("KDM:") + "\n")
		rightBuilder.WriteString(m.details.KDMURL + "\n\n")
		rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Image lists (k3s-images.txt, rke2-images-*.txt):") + "\n")
		rightBuilder.WriteString(m.details.ImageListSource + "\n\n")
		rightBuilder.WriteString(strings.Repeat("─", rightWidth-2) + "\n\n")
	}

	// Collect selected cluster types and CNI
	var selectedClusters []string
	var selectedCNI string
	for i, r := range m.rows {
		if m.selected[i] {
			if r.kind == "cluster" {
				selectedClusters = append(selectedClusters, r.label)
			}
			if r.kind == "cni" {
				selectedCNI = r.label
			}
		}
	}

	linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Underline(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	if isLBStage {
		rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Load balancer / Ingress") + "\n\n")
		rightBuilder.WriteString("K3s Klipper – Service load balancer (klipper-helm, klipper-lb). Default in K3s.\n\n")
		rightBuilder.WriteString("K3s Traefik – Ingress controller (default in K3s). Multi-arch.\n\n")
		rightBuilder.WriteString("RKE2 NGINX – NGINX Ingress Controller (default in RKE2).\n\n")
		rightBuilder.WriteString("RKE2 Traefik – Traefik ingress alternative for RKE2.\n\n")
		rightBuilder.WriteString(dimStyle.Render("Space toggles each. Include only the LBs you need.") + "\n\n")
		rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Documentation:") + "\n")
		rightBuilder.WriteString("RKE2 networking: " + linkStyle.Render(docRKE2Networking) + "\n")
		rightBuilder.WriteString("K3s networking:  " + linkStyle.Render(docK3sNetworking) + "\n")
		rightBuilder.WriteString("Traefik:        " + linkStyle.Render(docTraefik) + "\n")
		rightBuilder.WriteString("NGINX Ingress:  " + linkStyle.Render(docNGINXIngress) + "\n")
	} else if len(selectedClusters) > 0 {
		rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Selected Components") + "\n\n")
		rightBuilder.WriteString("Distro Images:\n")
		for _, cluster := range selectedClusters {
			rightBuilder.WriteString("  • " + cluster + " core images\n")
			rightBuilder.WriteString("    (control-plane, system components)\n")
		}
		if isCNIStage && selectedCNI != "" && selectedCNI != "CNI: None" {
			cniName := selectedCNI
			if strings.HasPrefix(selectedCNI, "CNI: ") {
				cniName = strings.TrimPrefix(selectedCNI, "CNI: ")
			} else if selectedCNI == "cni_canal" {
				cniName = "Canal"
			} else if selectedCNI == "cni_calico" {
				cniName = "Calico"
			} else if selectedCNI == "cni_cilium" {
				cniName = "Cilium"
			} else if selectedCNI == "cni_flannel" {
				cniName = "Flannel"
			}
			rightBuilder.WriteString("\nCNI Images:\n  • " + cniName + " CNI images\n")
		}
		rightBuilder.WriteString("\n" + dimStyle.Render("(Exact images depend on selected Kubernetes versions)") + "\n")
		if isDistroStage {
			rightBuilder.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Documentation") + "\n")
			rightBuilder.WriteString("Rancher: " + linkStyle.Render(docRancherManager) + "\n")
			rightBuilder.WriteString("K3s:    " + linkStyle.Render(docK3s) + "  RKE2: " + linkStyle.Render(docRKE2) + "\n")
		}
	} else {
		if isDistroStage {
			rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Distros") + "\n\n")
			rightBuilder.WriteString("K3s  – Lightweight, CNCF-certified Kubernetes (SUSE/Rancher). Edge & IoT.\n")
			rightBuilder.WriteString("      " + linkStyle.Render(docK3s) + "\n\n")
			rightBuilder.WriteString("RKE2 – Rancher Kubernetes Engine 2. CIS hardened, FIPS.\n")
			rightBuilder.WriteString("      " + linkStyle.Render(docRKE2) + "\n\n")
			if m.showRKE1 {
				rightBuilder.WriteString("RKE1 – Legacy RKE (EOL; prefer RKE2).\n")
				rightBuilder.WriteString("      " + linkStyle.Render(docRKE1) + "\n\n")
			}
			rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Platform") + "\n\n")
			rightBuilder.WriteString("Linux only – No Windows node images.\n\n")
			rightBuilder.WriteString("Linux + Windows – Include RKE2/K3s Windows node images (Calico or Flannel CNI required for Windows).\n")
			rightBuilder.WriteString("      " + linkStyle.Render(docRKE2Windows) + "\n\n")
			rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Documentation") + "\n")
			rightBuilder.WriteString("Rancher Manager: " + linkStyle.Render(docRancherManager) + "\n")
			rightBuilder.WriteString("Quick start K3s: " + linkStyle.Render(docK3sInstall) + "\n")
			rightBuilder.WriteString("Quick start RKE2: " + linkStyle.Render(docRKE2Install) + "\n")
		} else {
			rightBuilder.WriteString("Select options in the left column\nto see details.\n")
		}
	}

	// Add stage-specific docs when no selection yet (CNI stage) or for CNI stage in all cases
	if isCNIStage {
		if len(selectedClusters) == 0 {
			rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("CNI options") + "\n\n")
			rightBuilder.WriteString("Canal   – Calico + Flannel. Policy + overlay. (RKE2/K3s)\n")
			rightBuilder.WriteString("Calico  – Policy & networking. BGP, eBPF. (RKE2/K3s; required for Windows)\n")
			rightBuilder.WriteString("Cilium  – eBPF, observability, multi-cluster. (RKE2/K3s)\n")
			rightBuilder.WriteString("Flannel – Simple overlay. K3s only.\n\n")
			rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Documentation") + "\n")
			rightBuilder.WriteString("Rancher CNI FAQ: " + linkStyle.Render(docRancherCNI) + "\n")
			rightBuilder.WriteString("Calico:         " + linkStyle.Render(docCalico) + "\n")
			rightBuilder.WriteString("Cilium:         " + linkStyle.Render(docCilium) + "\n")
			rightBuilder.WriteString("Flannel:        " + linkStyle.Render(docFlannel) + "\n")
		} else {
			rightBuilder.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Documentation") + "\n")
			rightBuilder.WriteString("Rancher CNI: " + linkStyle.Render(docRancherCNI) + "\n")
			rightBuilder.WriteString("K3s net:    " + linkStyle.Render(docK3sNetworking) + "\n")
		}
	}

	// Style columns; reserve last line for selection footer
	leftStyle := lipgloss.NewStyle().Width(leftWidth)
	rightStyle := lipgloss.NewStyle().Width(rightWidth).Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("8"))
	if m.height > 2 {
		leftStyle = leftStyle.MaxHeight(m.height - 2)
		rightStyle = rightStyle.MaxHeight(m.height - 2)
	}
	leftContent := leftStyle.Render(leftBuilder.String())
	rightContent := rightStyle.Render(rightBuilder.String())
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftContent, rightContent)
	// Selection footer: past steps (if any) + current step selection
	var selParts []string
	for i, r := range m.rows {
		if m.selected[i] {
			selParts = append(selParts, r.label)
		}
	}
	sel := "(none)"
	if len(selParts) > 0 {
		sel = strings.Join(selParts, ", ")
		if width > 0 && len(sel) > width-16 {
			sel = sel[:width-19] + "..."
		}
	}
	footerLine := "Selection: " + sel
	if m.pastSelection != "" {
		footerLine = "Selection: " + m.pastSelection + "  →  " + sel
		if width > 0 && len(footerLine) > width-2 {
			maxPast := width - len("Selection: ") - len("  →  ") - 24
			if maxPast > 0 && len(m.pastSelection) > maxPast {
				footerLine = "Selection: " + m.pastSelection[:maxPast-3] + "...  →  " + sel
			}
			if len(footerLine) > width-2 {
				maxCur := width - len("Selection: ") - len("  →  ") - 20
				if maxCur > 0 && len(sel) > maxCur {
					sel = sel[:maxCur-3] + "..."
					footerLine = "Selection: " + m.pastSelection + "  →  " + sel
				}
			}
		}
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render(footerLine)
	return lipgloss.JoinVertical(lipgloss.Left, mainContent, footer)
}

// LBOptions holds per-distro load balancer choices from Step 1.
type LBOptions struct {
	K3sKlipper  bool // K3s: Klipper service LB
	K3sTraefik  bool // K3s: Traefik ingress
	RKE2Nginx   bool // RKE2: NGINX Ingress
	RKE2Traefik bool // RKE2: Traefik ingress
}

// formatStep1PastDistro returns a short summary of distro + platform for the selection footer.
func formatStep1PastDistro(selectedDistros []string, includeWindows bool) string {
	var labels []string
	for _, d := range selectedDistros {
		switch d {
		case "k3s":
			labels = append(labels, "K3s")
		case "rke2":
			labels = append(labels, "RKE2")
		case "rke":
			labels = append(labels, "RKE1")
		default:
			labels = append(labels, d)
		}
	}
	s := strings.Join(labels, ", ")
	if includeWindows {
		s += "; Linux + Windows"
	} else {
		s += "; Linux only"
	}
	return s
}

// formatStep1PastCNI returns a short label for the CNI id.
func formatStep1PastCNI(cniID string) string {
	switch cniID {
	case "cni_canal":
		return "canal"
	case "cni_calico":
		return "calico"
	case "cni_cilium":
		return "cilium"
	case "cni_flannel":
		return "flannel"
	}
	return strings.TrimPrefix(cniID, "cni_")
}

// formatStep1PastLB returns a short summary of load balancer choices for the selection footer.
func formatStep1PastLB(opts LBOptions) string {
	var parts []string
	if opts.K3sKlipper {
		parts = append(parts, "Klipper")
	}
	if opts.K3sTraefik {
		parts = append(parts, "Traefik(K3s)")
	}
	if opts.RKE2Nginx {
		parts = append(parts, "NGINX")
	}
	if opts.RKE2Traefik {
		parts = append(parts, "Traefik(RKE2)")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// RunStep1TUI runs the Step 1 TUI in stages: distro (+ platform) → CNI → load balancer → versions.
// hasRKE1 controls whether RKE1 is shown.
// capabilities provides Kubernetes versions for each cluster type.
// details is shown in the right panel (KDM URL, image list source).
// Returns components, k3sVers, rke2Vers, rkeVers, cni, lbOpts, includeWindows (include Windows node images), err.
func RunStep1TUI(hasRKE1 bool, capabilities map[string]kdmimages.ClusterVersionInfo, details Step1Details) (components string, k3sVers string, rke2Vers string, rkeVers string, cni string, lbOpts LBOptions, includeWindows bool, err error) {
	// Stage 1: Select distro (K3s, RKE2, RKE1) + platform (Linux only / Linux + Windows)
	var distroRows []step1Row
	distroRows = append(distroRows, step1Row{"cluster", "k3s", "K3s"})
	distroRows = append(distroRows, step1Row{"cluster", "rke2", "RKE2"})
	if hasRKE1 {
		distroRows = append(distroRows, step1Row{"cluster", "rke", "RKE1"})
	}
	distroRows = append(distroRows, step1Row{"platform", "linux_only", "Linux only"})
	distroRows = append(distroRows, step1Row{"platform", "linux_windows", "Linux + Windows (RKE2/K3s Windows node images)"})

	distroSelected := make(map[int]bool)
	for i := range distroRows {
		distroSelected[i] = false
	}
	// Default: select K3s and RKE2
	for i, r := range distroRows {
		if r.id == "k3s" || r.id == "rke2" {
			distroSelected[i] = true
		}
	}
	// Default: Linux only (first platform option)
	for i, r := range distroRows {
		if r.kind == "platform" && r.id == "linux_only" {
			distroSelected[i] = true
			break
		}
	}

	distroModel := step1Model{
		rows:     distroRows,
		cursor:   0,
		selected: distroSelected,
		done:     false,
		showRKE1: hasRKE1,
		details:  details,
	}

	var selectedDistros []string
	var includeWin bool
	var cniSel string

	// Distro + CNI: loop until user confirms CNI (doesn't press 'b' to go back).
	// LB and version loops are inside so that "b" from LB→CNI can continue to distro.
distroLoop:
	for {
		p1 := tea.NewProgram(distroModel)
		final1, err := p1.Run()
		if err != nil {
			return "", "", "", "", "", LBOptions{}, false, err
		}
		mm1 := final1.(step1Model)
		if mm1.aborted {
			return "", "", "", "", "", LBOptions{}, false, ErrAborted
		}
		selectedDistros = nil
		includeWin = false
		for i, r := range mm1.rows {
			if r.kind == "platform" && mm1.selected[i] && r.id == "linux_windows" {
				includeWin = true
			}
			if r.kind == "cluster" && mm1.selected[i] {
				selectedDistros = append(selectedDistros, r.id)
			}
		}
		if len(selectedDistros) == 0 {
			return "", "", "", "", "", LBOptions{}, false, fmt.Errorf("at least one distro must be selected")
		}

		// Stage 2: Select CNI (based on selected distros - Flannel only for K3s)
		var cniRows []step1Row
		cniRows = append(cniRows, step1Row{"cni", "cni_canal", "canal"})
		cniRows = append(cniRows, step1Row{"cni", "cni_calico", "calico"})
		cniRows = append(cniRows, step1Row{"cni", "cni_cilium", "cilium"})
		hasK3sForCNI := false
		for _, d := range selectedDistros {
			if d == "k3s" {
				hasK3sForCNI = true
				break
			}
		}
		if hasK3sForCNI {
			cniRows = append(cniRows, step1Row{"cni", "cni_flannel", "flannel"})
		}
		cniSelected := make(map[int]bool)
		for i := range cniRows {
			cniSelected[i] = false
		}
		if len(cniRows) > 0 {
			cniSelected[0] = true
		}
		pastDistro := formatStep1PastDistro(selectedDistros, includeWin)
		cniModel := step1Model{
			rows:          cniRows,
			cursor:        0,
			selected:      cniSelected,
			done:          false,
			showRKE1:      hasRKE1,
			details:       details,
			pastSelection: pastDistro,
		}
		p2 := tea.NewProgram(cniModel)
		final2, err := p2.Run()
		if err != nil {
			return "", "", "", "", "", LBOptions{}, false, err
		}
		mm2 := final2.(step1Model)
		if mm2.aborted {
			return "", "", "", "", "", LBOptions{}, false, ErrAborted
		}
		cniSel = ""
		for i, r := range mm2.rows {
			if mm2.selected[i] {
				cniSel = r.id
				break
			}
		}
		if cniSel == "" {
			cniSel = "cni_canal"
		}
		if mm2.wantBack {
			// User pressed 'b' on CNI: re-run distro (and CNI) with current distro pre-selected
			distroSelected = make(map[int]bool)
			for i := range distroRows {
				distroSelected[i] = mm1.selected[i]
			}
			distroModel = step1Model{rows: distroRows, cursor: 0, selected: distroSelected, done: false, showRKE1: hasRKE1, details: details}
			continue distroLoop
		}
		// CNI confirmed; run LB then version selection then return

		// Stage 2b: Load balancer – loop until user confirms (or 'b' re-runs CNI then LB again)
		hasK3s := false
		hasRKE2 := false
		for _, d := range selectedDistros {
			if d == "k3s" {
				hasK3s = true
			}
			if d == "rke2" {
				hasRKE2 = true
			}
		}
		for {
			var lbRows []step1Row
			if hasK3s {
				lbRows = append(lbRows, step1Row{kind: "lb", id: "k3s_klipper", label: "K3s: Klipper (service LB)"})
				lbRows = append(lbRows, step1Row{kind: "lb", id: "k3s_traefik", label: "K3s: Traefik (ingress)"})
			}
			if hasRKE2 {
				lbRows = append(lbRows, step1Row{kind: "lb", id: "rke2_nginx", label: "RKE2: NGINX Ingress"})
				lbRows = append(lbRows, step1Row{kind: "lb", id: "rke2_traefik", label: "RKE2: Traefik (ingress)"})
			}
			lbSelected := make(map[int]bool)
			for i := range lbRows {
				lbSelected[i] = true
			}
			pastCNI := pastDistro + "; CNI: " + formatStep1PastCNI(cniSel)
			lbModel := step1Model{
				rows:          lbRows,
				cursor:        0,
				selected:      lbSelected,
				done:          false,
				showRKE1:      hasRKE1,
				details:       details,
				pastSelection: pastCNI,
			}
			p2b := tea.NewProgram(lbModel)
			final2b, err := p2b.Run()
			if err != nil {
				return "", "", "", "", "", LBOptions{}, false, err
			}
			mm2b := final2b.(step1Model)
			if mm2b.aborted {
				return "", "", "", "", "", LBOptions{}, false, ErrAborted
			}
			lbOpts = LBOptions{K3sKlipper: false, K3sTraefik: false, RKE2Nginx: false, RKE2Traefik: false}
			for i, row := range mm2b.rows {
				if !mm2b.selected[i] {
					continue
				}
				switch row.id {
				case "k3s_klipper":
					lbOpts.K3sKlipper = true
				case "k3s_traefik":
					lbOpts.K3sTraefik = true
				case "rke2_nginx":
					lbOpts.RKE2Nginx = true
				case "rke2_traefik":
					lbOpts.RKE2Traefik = true
				}
			}
			if !mm2b.wantBack {
				break
			}
			// User pressed 'b' on LB: re-run CNI then LB again
			cniRows := []step1Row{
				{"cni", "cni_canal", "canal"},
				{"cni", "cni_calico", "calico"},
				{"cni", "cni_cilium", "cilium"},
			}
			if hasK3s {
				cniRows = append(cniRows, step1Row{"cni", "cni_flannel", "flannel"})
			}
			cniSelected := make(map[int]bool)
			for i := range cniRows {
				cniSelected[i] = (i == 0)
			}
			cniModel := step1Model{rows: cniRows, cursor: 0, selected: cniSelected, done: false, aborted: false, showRKE1: hasRKE1, details: details}
			p2 := tea.NewProgram(cniModel)
			final2, err := p2.Run()
			if err != nil {
				return "", "", "", "", "", LBOptions{}, false, err
			}
			mm2 := final2.(step1Model)
			if mm2.aborted {
				return "", "", "", "", "", LBOptions{}, false, ErrAborted
			}
			cniSel = "cni_canal"
			for i, r := range mm2.rows {
				if mm2.selected[i] {
					cniSel = r.id
					break
				}
			}
			if mm2.wantBack {
				// User pressed 'b' on CNI when re-running from LB: go back to distro step
				continue distroLoop
			}
		}

		// Stage 3: Version selection for each selected cluster type; 'b' re-shows LB then versions again
		k3sVers := "all"
		rke2Vers := "all"
		rkeVers := "all"
		pastLB := formatStep1PastDistro(selectedDistros, includeWin) + "; CNI: " + formatStep1PastCNI(cniSel) + "; LB: " + formatStep1PastLB(lbOpts)
	versionLoop:
		for {
			for _, comp := range selectedDistros {
				info, ok := capabilities[comp]
				if !ok || len(info.Versions) == 0 {
					continue
				}
				versions := info.Versions
				selectedVers, wantBack, verr := runVersionSelectionTUI(comp, versions, pastLB)
				if verr != nil {
					return "", "", "", "", "", LBOptions{}, false, verr
				}
				if wantBack {
					// Re-show LB step, then re-run version selection from start
					var lbRows []step1Row
					if hasK3s {
						lbRows = append(lbRows, step1Row{kind: "lb", id: "k3s_klipper", label: "K3s: Klipper (service LB)"})
						lbRows = append(lbRows, step1Row{kind: "lb", id: "k3s_traefik", label: "K3s: Traefik (ingress)"})
					}
					if hasRKE2 {
						lbRows = append(lbRows, step1Row{kind: "lb", id: "rke2_nginx", label: "RKE2: NGINX Ingress"})
						lbRows = append(lbRows, step1Row{kind: "lb", id: "rke2_traefik", label: "RKE2: Traefik (ingress)"})
					}
					lbSelected := make(map[int]bool)
					for i := range lbRows {
						lbSelected[i] = true
					}
					pastCNI := formatStep1PastDistro(selectedDistros, includeWin) + "; CNI: " + formatStep1PastCNI(cniSel)
					lbModel := step1Model{rows: lbRows, cursor: 0, selected: lbSelected, done: false, showRKE1: hasRKE1, details: details, pastSelection: pastCNI}
					p2b := tea.NewProgram(lbModel)
					final2b, err := p2b.Run()
					if err != nil {
						return "", "", "", "", "", LBOptions{}, false, err
					}
					mm2b := final2b.(step1Model)
					lbOpts = LBOptions{}
					for i, row := range mm2b.rows {
						if !mm2b.selected[i] {
							continue
						}
						switch row.id {
						case "k3s_klipper":
							lbOpts.K3sKlipper = true
						case "k3s_traefik":
							lbOpts.K3sTraefik = true
						case "rke2_nginx":
							lbOpts.RKE2Nginx = true
						case "rke2_traefik":
							lbOpts.RKE2Traefik = true
						}
					}
					continue versionLoop
				}
				if len(selectedVers) > 0 {
					versStr := strings.Join(selectedVers, ",")
					switch comp {
					case "k3s":
						k3sVers = versStr
					case "rke2":
						rke2Vers = versStr
					case "rke":
						rkeVers = versStr
					}
				}
			}
			break
		}

		return strings.Join(selectedDistros, ","), k3sVers, rke2Vers, rkeVers, cniSel, lbOpts, includeWin, nil
	}
}

// versionSelectionModel is a TUI for selecting Kubernetes versions.
type versionSelectionModel struct {
	clusterType   string
	versions      []string
	cursor        int
	selected      map[int]bool
	done          bool
	wantBack      bool   // true when user pressed 'b' to return to previous step
	aborted       bool   // true when user pressed q or Ctrl+C to exit immediately
	pastSelection string // summary of Step 1 choices (distro, CNI, LB) for footer
}

func (m *versionSelectionModel) Init() tea.Cmd {
	return nil
}

func (m *versionSelectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.versions)-1 {
				m.cursor++
			}
			return m, nil
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
			return m, nil
		case "a":
			// Select all
			for i := range m.versions {
				m.selected[i] = true
			}
			return m, nil
		case "enter", "d":
			m.done = true
			return m, tea.Quit
		case "b", "B":
			m.wantBack = true
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *versionSelectionModel) View() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render
	b.WriteString(title(fmt.Sprintf("Step 2.4: Select %s Kubernetes versions", strings.ToUpper(m.clusterType))) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true).Render("Select which Kubernetes versions to include. Only selected versions will appear in the image list.") + "\n\n")
	b.WriteString("↑/↓ move   Space toggle   a select all   Enter/d done   b back   q quit   Ctrl+C exit\n\n")
	for i, v := range m.versions {
		prefix := "  "
		if m.selected[i] {
			prefix = "X "
		}
		line := prefix + v
		if i == m.cursor {
			// Preserve the selection symbol when cursor is on the line
			line = "▸ " + lipgloss.NewStyle().Bold(true).Render(line)
		} else {
			// Remove leading spaces but keep selection symbol
			if strings.HasPrefix(line, "  ") {
				line = line[2:]
			}
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\nPress Enter or 'd' when done, or 'b' to go back to the previous step (or 'a' to select all, empty = all versions).\n")
	var selParts []string
	for i, v := range m.versions {
		if m.selected[i] {
			selParts = append(selParts, v)
		}
	}
	sel := "(all)"
	if len(selParts) > 0 {
		sel = strings.Join(selParts, ", ")
		if len(sel) > 60 {
			sel = sel[:57] + "..."
		}
	}
	footerLine := "Selection: " + sel
	if m.pastSelection != "" {
		footerLine = "Selection: " + m.pastSelection + "  →  " + strings.ToUpper(m.clusterType) + ": " + sel
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render(footerLine)
	return b.String() + "\n" + footer
}

// runVersionSelectionTUI shows a TUI for selecting Kubernetes versions for a cluster type.
// pastSelection is the Step 1 summary (distro, CNI, LB) to show in the footer.
// Returns selected versions, wantBack (true if user pressed 'b' to return to previous step), and error.
func runVersionSelectionTUI(clusterType string, versions []string, pastSelection string) ([]string, bool, error) {
	if len(versions) == 0 {
		return nil, false, nil
	}

	selected := make(map[int]bool)
	for i := range versions {
		selected[i] = false // Default: none selected = use "all"
	}

	m := &versionSelectionModel{
		clusterType:   clusterType,
		versions:      versions,
		cursor:        0,
		selected:      selected,
		done:          false,
		wantBack:      false,
		aborted:       false,
		pastSelection: pastSelection,
	}

	// Don't use alt screen so logs remain visible
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, false, err
	}
	mm := final.(*versionSelectionModel)
	if mm.aborted {
		return nil, false, ErrAborted
	}
	if mm.wantBack {
		return nil, true, nil
	}

	var selectedVers []string
	hasSelection := false
	for i := range mm.versions {
		if mm.selected[i] {
			hasSelection = true
			selectedVers = append(selectedVers, mm.versions[i])
		}
	}

	// If nothing selected, return empty = use "all"
	if !hasSelection {
		return nil, false, nil
	}
	return selectedVers, false, nil
}
