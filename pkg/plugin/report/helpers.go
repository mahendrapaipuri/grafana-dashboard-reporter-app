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

// selectPanels returns panel indexes to render based on IncludePanelIDs and ExcludePanelIDs
// config parameters.
func selectPanels(panels []dashboard.Panel, includeIDs, excludeIDs []string, defaultInclude bool) []int {
	var renderPanels []int

	// If includeIDs is empty and default behaviour is to include all, setuo
	// includeIDs
	if len(includeIDs) == 0 && defaultInclude {
		for _, p := range panels {
			includeIDs = append(includeIDs, p.ID)
		}
	}

	for iPanel, panel := range panels {
		// Attempt to convert panel ID to int. If we succeed, do direct
		// comparison else do prefix check
		var doDirectComp bool
		if _, err := strconv.ParseInt(panel.ID, 10, 0); err == nil {
			doDirectComp = true
		}

		for _, id := range includeIDs {
			if !doDirectComp {
				if strings.HasPrefix(panel.ID, id) && !slices.Contains(renderPanels, iPanel) {
					renderPanels = append(renderPanels, iPanel)
				}
			} else {
				if panel.ID == id && !slices.Contains(renderPanels, iPanel) {
					renderPanels = append(renderPanels, iPanel)
				}
			}
		}

		exclude := false

		for _, id := range excludeIDs {
			if !doDirectComp {
				if strings.HasPrefix(panel.ID, id) {
					exclude = true
				}
			} else {
				if panel.ID == id {
					exclude = true
				}
			}
		}

		if exclude && slices.Contains(renderPanels, iPanel) {
			renderPanels = remove(renderPanels, iPanel)
		}
	}

	return renderPanels
}
