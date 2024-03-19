package plugin

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type PanelType int

func (p PanelType) string() string {
	return [...]string{
		"singlestat",
		"text",
		"graph",
		"table",
	}[p]
}

const (
	SingleStat PanelType = iota
	Text
	Graph
	Table
)

// Panel represents a Grafana dashboard panel position
type GridPos struct {
	H float64 `json:"h"`
	W float64 `json:"w"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Panel represents a Grafana dashboard panel
type Panel struct {
	Id      int     `json:"id"`
	Type    string  `json:"type"`
	Title   string  `json:"title"`
	GridPos GridPos `json:"gridPos"`
}

// Is panel single stat?
func (p Panel) IsSingleStat() bool {
	return p.Is(SingleStat)
}

// If panel has width less than total allowable width
func (p Panel) IsPartialWidth() bool {
	return (p.GridPos.W < 24)
}

// Get panel width
func (p Panel) Width() float64 {
	return float64(p.GridPos.W) * 0.04
}

// Get panel height
func (p Panel) Height() float64 {
	return float64(p.GridPos.H) * 0.04
}

// If panel is of type
func (p Panel) Is(t PanelType) bool {
	return p.Type == t.string()
}

// Row represents a container for Panels
type RowOrPanel struct {
	Panel
	Collapsed bool    `json:"collapsed"`
	Panels    []Panel `json:"panels"`
}

// Dashboard represents a Grafana dashboard
// This is both used to unmarshal the dashboard JSON into
// and then enriched (sanitize fields for TeX consumption and add VarialbeValues)
type Dashboard struct {
	Title          string       `json:"title"`
	Description    string       `json:"description"`
	VariableValues string       // Not present in the Grafana JSON structure. Enriched data passed used by the Tex templating
	RowOrPanels    []RowOrPanel `json:"panels"`
	Panels         []Panel
}

// Get dashboard variables
func getVariablesValues(queryParams url.Values) string {
	values := []string{}
	for k, v := range queryParams {
		if strings.HasPrefix(k, "var-") {
			n := strings.Split(k, "var-")[1]
			values = append(values, fmt.Sprintf("%s=%s", n, strings.Join(v, ",")))
		}
	}
	return strings.Join(values, "; ")
}

// NewDashboard creates Dashboard from Grafana's internal JSON dashboard definition
func NewDashboard(dashJSON []byte, queryParams url.Values, dashboardMode string) Dashboard {
	var dash map[string]Dashboard
	if err := json.Unmarshal(dashJSON, &dash); err != nil {
		panic(err)
	}

	// Get dashboard from map
	if dashboard, ok := dash["dashboard"]; !ok {
		panic(fmt.Errorf("dashboard model missing"))
	} else {
		// Remove row panels from model
		var filteredPanels []Panel
		// In the case of collapsed rows, the gridPos within the row will not be
		// consistent with gridPos of dashboard. As rows are collapsed the "y" ordinate
		// within row with have higher value than "y" ordinate of global dashboard.
		// We will need to account it when report of "full" dashboard is requested.
		var globalYPos float64
		var globalYPosHeight float64
		for _, p := range dashboard.RowOrPanels {
			// If the panel is of type row and there are panels inside the row
			if p.Type == "row" {
				// If default dashboard is requested and panels are collapsed in dashboard
				// skip finding collapsed panels
				if dashboardMode == "default" && p.Collapsed {
					continue
				}

				// In other cases, find all collapsed panels and add them to final panel list
				var startYPos float64
				for irp, rp := range p.Panels {
					// State variable for the mark of start of row y position
					if irp == 0 {
						startYPos = rp.GridPos.Y
					}

					// If it is a collapsed row the gridPos of panels inside row will
					// be relatively placed to gridPos of row.
					// Here we transform those relative gridPos into absolute by using
					// last y position of panel before row and start of first panel inside
					// the rwo
					if p.Collapsed {
						rp.GridPos.Y = rp.GridPos.Y - startYPos + globalYPos + globalYPosHeight
					}

					// Update the y position using last panel of the row
					if irp == len(p.Panels)-1 {
						globalYPos = rp.GridPos.Y
						globalYPosHeight = rp.GridPos.H
					}
					filteredPanels = append(filteredPanels, rp)
				}
				continue
			}

			// Once a row has been created, all the panels below the row will be
			// encapsulated into row. So, there cant be standalone panels **after** rows.
			// Hence get the **last** y position and height of last panel before we
			// get rows.
			globalYPos = p.Panel.GridPos.Y
			globalYPosHeight = p.Panel.GridPos.H
			filteredPanels = append(filteredPanels, p.Panel)
		}
		dashboard.Panels = filteredPanels
		dashboard.VariableValues = getVariablesValues(queryParams)
		return dashboard
	}
}
