package report

import (
	"slices"
	"strconv"
	"strings"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
)

// remove removes a element by value in slice and returns a new slice.
func remove[T comparable](l []T, item T) []T {
	out := make([]T, 0)

	for _, element := range l {
		if element != item {
			out = append(out, element)
		}
	}

	return out
}

// selectPanels returns panel indexes to render based on IncludePanelIDs and ExcludePanelIDs
// config parameters.
func selectPanels(panels []dashboard.Panel, includeIDs, excludeIDs []string, defaultInclude bool) []int {
	var renderPanels []int

	// If includeIDs is empty and default behaviour is to include all, setuo
	// includeIDs
	if len(includeIDs) == 0 && defaultInclude {
		for _, p := range panels {
			includeIDs = append(includeIDs, strings.Split(p.ID, "-clone")[0])
		}
	}

	for iPanel, panel := range panels {
		// Attempt to convert panel ID to int. If we succeed, do direct
		// comparison else do prefix check
		panelID := panel.ID
		if _, err := strconv.ParseInt(panel.ID, 10, 0); err != nil {
			panelID = strings.Split(panel.ID, "-clone")[0]
		}

		for _, id := range includeIDs {
			if panelID == id && !slices.Contains(renderPanels, iPanel) {
				renderPanels = append(renderPanels, iPanel)
			}
		}

		if slices.Contains(excludeIDs, panelID) && slices.Contains(renderPanels, iPanel) {
			renderPanels = remove(renderPanels, iPanel)
		}
	}

	return renderPanels
}
