package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Regex for parsing X and Y co-ordinates from CSS
// Scales for converting width and height to Grafana units
var (
	translateRegex = regexp.MustCompile(`translate\((?P<X>\d+)px, (?P<Y>\d+)px\)`)
	scales         = map[string]int{
		"width":  30,
		"height": 36,
	}
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
	ID      int     `json:"id"`
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
func variablesValues(queryParams url.Values) string {
	values := []string{}
	for k, v := range queryParams {
		if strings.HasPrefix(k, "var-") {
			n := strings.Split(k, "var-")[1]
			values = append(values, fmt.Sprintf("%s=%s", n, strings.Join(v, ",")))
		}
	}
	return strings.Join(values, "; ")
}

// NewDashboard creates Dashboard from Grafana's internal JSON dashboard model
// fetched from Grafana API and browser
func NewDashboard(dashJSON []byte, dashData []interface{}, queryParams url.Values, config *Config) (Dashboard, error) {
	var dash map[string]Dashboard
	if err := json.Unmarshal(dashJSON, &dash); err != nil {
		return Dashboard{}, err
	}

	// Get dashboard from map
	if dashboard, ok := dash["dashboard"]; !ok {
		return Dashboard{}, fmt.Errorf("no dashboard model found in Grafana API response")
	} else {
		// Attempt to update panels from browser data
		// If there are no errors, update the panels from browser dashabord model and
		// return
		var panels []Panel
		var err error
		if panels, err = panelsFromBrowser(dashData); err != nil {
			// If we fail to get panels from browser data, get them from dashboard JSON model
			// and correct grid positions
			panels = panelsFromJSON(dashboard.RowOrPanels, config.DashboardMode)
		}

		// Filter the panels based on IncludePanelIDs/ExcludePanelIDs
		dashboard.Panels = filterPanels(panels, config)
		// Add query parameters to dashboard model
		dashboard.VariableValues = variablesValues(queryParams)
		return dashboard, err
	}
}

// panelsFromBrowser creates slice of panels from the data fetched from browser's DOM model
func panelsFromBrowser(dashData []interface{}) ([]Panel, error) {
	// If dashData is nil return
	if dashData == nil {
		return nil, fmt.Errorf("no dashboard data found in browser data")
	}

	var panels []Panel
	var allErrs error
	var err error
	// Iterate over the slice of interfaces and build each panel
	for _, p := range dashData {
		var id, x, y, w, h, vInt, xInt, yInt int
		for k, v := range p.(map[string]interface{}) {
			switch k {
			case "width":
				if vInt, err = strconv.Atoi(strings.TrimSuffix(v.(string), "px")); err != nil {
					allErrs = errors.Join(err, allErrs)
				}
				w = vInt / scales[k]
			case "height":
				if vInt, err = strconv.Atoi(strings.TrimSuffix(v.(string), "px")); err != nil {
					allErrs = errors.Join(err, allErrs)
				}
				h = vInt / scales[k]
			case "transform":
				matches := translateRegex.FindStringSubmatch(v.(string))
				if len(matches) == 3 {
					xCoord := matches[translateRegex.SubexpIndex("X")]
					if xInt, err = strconv.Atoi(xCoord); err != nil {
						allErrs = errors.Join(err, allErrs)
					} else {
						x = xInt / scales["width"]
					}
					yCoord := matches[translateRegex.SubexpIndex("Y")]
					if yInt, err = strconv.Atoi(yCoord); err != nil {
						allErrs = errors.Join(err, allErrs)
					} else {
						y = yInt / scales["height"]
					}
				} else {
					allErrs = errors.Join(fmt.Errorf("failed to capture X and Y coordinate regex groups"), allErrs)
				}
			case "id":
				if id, err = strconv.Atoi(v.(string)); err != nil {
					allErrs = errors.Join(err, allErrs)
				}
			}
		}

		// If height comes to zero, it is row panel and ignore it
		if h == 0 {
			continue
		}

		// Create panel model and append to panels
		panels = append(panels, Panel{
			ID: id,
			GridPos: GridPos{
				X: float64(x),
				Y: float64(y),
				H: float64(h),
				W: float64(w),
			},
		})
	}

	// Check if we fetched any panels
	if len(panels) == 0 {
		allErrs = errors.Join(err, fmt.Errorf("no panels found in browser data"))
		return nil, allErrs
	}
	return panels, allErrs
}

// panelsFromJSON makes panels from dashboard JSON model by uncollapsing and correcting
// grid positions for all row panels when dashboardMode is full
func panelsFromJSON(rowOrPanels []RowOrPanel, dashboardMode string) []Panel {
	// In the case of collapsed rows, the gridPos within the row will not be
	// consistent with gridPos of dashboard. As rows are collapsed the "y" ordinate
	// within row with have higher value than "y" ordinate of global dashboard.
	// We will need to account it when report of "full" dashboard is requested.
	var panels []Panel
	var globalYPos float64
	var globalYPosHeight float64
	for _, p := range rowOrPanels {
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
				panels = append(panels, rp)
			}
			continue
		}

		// Once a row has been created, all the panels below the row will be
		// encapsulated into row. So, there cant be standalone panels **after** rows.
		// Hence get the **last** y position and height of last panel before we
		// get rows.
		globalYPos = p.Panel.GridPos.Y
		globalYPosHeight = p.Panel.GridPos.H
		panels = append(panels, p.Panel)
	}
	return panels
}

// filterPanels filters the panels based on IncludePanelIDs and ExcludePanelIDs
// config parameters
func filterPanels(panels []Panel, config *Config) []Panel {
	// If config parameters are empty, return original panels
	if len(config.IncludePanelIDs) == 0 && len(config.ExcludePanelIDs) == 0 {
		return panels
	}

	// Iterate over all panels and check if they should be included or not
	var filteredPanels []Panel
	for _, panel := range panels {
		if len(config.IncludePanelIDs) > 0 && slices.Contains(config.IncludePanelIDs, panel.ID) &&
			!slices.Contains(filteredPanels, panel) {
			filteredPanels = append(filteredPanels, panel)
		}

		if len(config.ExcludePanelIDs) > 0 && !slices.Contains(config.ExcludePanelIDs, panel.ID) &&
			!slices.Contains(filteredPanels, panel) {
			filteredPanels = append(filteredPanels, panel)
		}
	}
	return filteredPanels
}
