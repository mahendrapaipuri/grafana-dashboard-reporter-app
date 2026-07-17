package report

import (
	"slices"
	"strconv"
	"strings"

	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
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

// normalizePanelID normalizes panel ID to be able to compare to user's input.
func normalizePanelID(panelID string) string {
	// Prior to Grafana 13.x, repeated panels used to have suffixes -clone-0, -clone-1, etc
	panelID = strings.Split(panelID, "-clone")[0]
	// From Grafana 13.x, we get panel IDs of format <var1>$<var2>$panel-3 for
	// repeated panels based on variables. If we find "$" in panelID, we split
	// it along $ and take the last value to get panel ID
	if strings.Contains(panelID, "$") {
		// Playground: https://go.dev/play/p/Jfj4FtbA2YF
		panelID = panelID[strings.LastIndex(panelID, "$")+1:]
	}

	return panelID
}

// selectPanels returns panel indexes to render based on IncludePanelIDs and ExcludePanelIDs
// config parameters.
func selectPanels(panels []dashboard.Panel, includeIDs, excludeIDs []string, defaultInclude bool) []int {
	var renderPanels []int

	// If includeIDs is empty and default behaviour is to include all, setuo
	// includeIDs
	if len(includeIDs) == 0 && defaultInclude {
		for _, p := range panels {
			includeIDs = append(includeIDs, normalizePanelID(p.ID))
		}
	}

	for iPanel, panel := range panels {
		// Attempt to convert panel ID to int. If we succeed, do direct
		// comparison else do prefix check
		panelID := panel.ID

		_, err := strconv.ParseInt(panel.ID, 10, 0)
		if err != nil {
			panelID = normalizePanelID(panel.ID)
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
