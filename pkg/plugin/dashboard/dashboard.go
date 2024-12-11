package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
)

// Regex for parsing X and Y co-ordinates from CSS
// Scales for converting width and height to Grafana units.
//
// This is based on viewportWidth that we used in client.go which
// is 1952px. Stripping margin 32px we get 1920px / 24 = 80px
// height scale should be fine with 36px as width and aspect ratio
// should choose a height appropriately.
var (
	scales = map[string]float64{
		"width":  80,
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

// CSVData represents type of the CSV data.
type CSVData [][]string

// GridPos represents a Grafana dashboard panel position.
type GridPos struct {
	H float64 `json:"h"`
	W float64 `json:"w"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type PanelID string

func (i *PanelID) UnmarshalJSON(b []byte) error {
	var item interface{}
	if err := json.Unmarshal(b, &item); err != nil {
		return err
	}

	switch v := item.(type) {
	case int:
		*i = PanelID(strconv.Itoa(v))
	case float64:
		*i = PanelID(strconv.Itoa(int(v)))
	case string:
		*i = PanelID(v)
	}

	return nil
}

// Panel represents a Grafana dashboard panel.
type Panel struct {
	ID           string  `json:"-"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	GridPos      GridPos `json:"gridPos"`
	EncodedImage PanelImage
	CSVData      CSVData
}

func (p *Panel) UnmarshalJSON(b []byte) error {
	type tmp Panel

	var s struct {
		tmp
		ID PanelID `json:"id"`
	}

	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	*p = Panel(s.tmp)
	p.ID = string(s.ID)

	return err
}

type PanelImage struct {
	Image    string
	MimeType string
}

func (p PanelImage) String() string {
	return fmt.Sprintf("data:%s;base64,%s", p.MimeType, p.Image)
}

// IsSingleStat returns true if panel is of type SingleStat.
func (p Panel) IsSingleStat() bool {
	return p.Is(SingleStat)
}

// IsPartialWidth If panel has width less than total allowable width.
func (p Panel) IsPartialWidth() bool {
	return (p.GridPos.W < 24)
}

// Width returns the width of the panel.
func (p Panel) Width() float64 {
	return float64(p.GridPos.W) * 0.04
}

// Height returns the height of the panel.
func (p Panel) Height() float64 {
	return float64(p.GridPos.H) * 0.04
}

// Is returns true if panel is of type t.
func (p Panel) Is(t PanelType) bool {
	return p.Type == t.string()
}

// RowOrPanel represents a container for Panels.
type RowOrPanel struct {
	Panel
	Collapsed bool    `json:"collapsed"`
	Panels    []Panel `json:"panels"`
}

// Dashboard represents a Grafana dashboard
// This is both used to unmarshal the dashboard JSON into
// and then enriched (sanitize fields for TeX consumption and add VarialbeValues).
type Dashboard struct {
	Title          string       `json:"title"`
	Description    string       `json:"description"`
	VariableValues string       // Not present in the Grafana JSON structure. Enriched data passed used by the Tex templating
	RowOrPanels    []RowOrPanel `json:"panels"`
	Panels         []Panel
}

// Get dashboard variables.
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

// New creates Dashboard from Grafana's internal JSON dashboard model
// fetched from Grafana API and browser.
func New(log log.Logger, config config.Config, dashJSON []byte, dashData []interface{}, queryParams url.Values) (Dashboard, error) {
	var dash map[string]Dashboard
	if err := json.Unmarshal(dashJSON, &dash); err != nil { //nolint:musttag
		return Dashboard{}, fmt.Errorf("failed to unmarshal dashboard JSON: %w", err)
	}

	// Get dashboard from map
	dashboard, ok := dash["dashboard"]
	if !ok {
		return Dashboard{}, fmt.Errorf("API response: %w", ErrNoDashboardData)
	}

	// Attempt to update panels from browser data

	var err error
	if dashboard.Panels, err = panelsFromBrowser(dashboard, dashData); err != nil {
		log.Warn("failed to get panels from browser data", "error", err)
		// If we fail to get panels from browser data, get them from dashboard JSON model
		// and correct grid positions
		dashboard.Panels = panelsFromJSON(dashboard.RowOrPanels, config.DashboardMode)
	}

	// Add query parameters to dashboard model
	dashboard.VariableValues = variablesValues(queryParams)

	return dashboard, err
}

// panelsFromBrowser creates slice of panels from the data fetched from browser's DOM model.
func panelsFromBrowser(dashboard Dashboard, dashData []interface{}) ([]Panel, error) {
	// If dashData is nil return
	if len(dashData) == 0 {
		return nil, fmt.Errorf("browser: %w", ErrNoDashboardData)
	}

	var (
		allErrs error
		err     error
		panels  []Panel
	)

	// We get HTML element's bounding box absolute coordinates which means
	// x and y start at non zero. We need to check those offsets and subtract
	// from all coordinates to ensure we start at (0, 0)
	xOffset := math.MaxFloat64
	yOffset := math.MaxFloat64

	// Seems like different versions of Grafana returns the max width differently.
	// So we check the maxWidth from returned coordinates and (w, h) tuples.
	// Max Width = Max X + Width for that element
	// We divide this maxWidth in 24 columns as done in Grafana to calculate Panel
	// coordinates
	var maxWidth float64

	// Iterate over the slice of interfaces and build each panel
	for _, panelData := range dashData {
		var p Panel

		pMap, ok := panelData.(map[string]interface{})
		if !ok {
			continue
		}

		for k, v := range pMap {
			switch v := v.(type) {
			case float64:
				switch k {
				case "width":
					p.GridPos.W = v
				case "height":
					p.GridPos.H = v
				case "x":
					p.GridPos.X = v

					if v < xOffset {
						xOffset = v
					}
				case "y":
					p.GridPos.Y = v

					if v < yOffset {
						yOffset = v
					}
				}
			case string:
				p.ID = v
			}

			if p.GridPos.X+p.GridPos.W > maxWidth {
				maxWidth = p.GridPos.X + p.GridPos.W
			}
		}

		// If height comes to 1 or less, it is row panel and ignore it
		if math.Round(p.GridPos.H/scales["height"]) <= 1 {
			continue
		}

		// Populate Type and Title from dashboard JSON model
		for _, rowOrPanel := range dashboard.RowOrPanels {
			if rowOrPanel.Type == "row" {
				for _, rp := range rowOrPanel.Panels {
					if rp.ID == p.ID {
						p.Type = rp.Type
						p.Title = rp.Title

						break
					}
				}
			} else {
				if p.ID == rowOrPanel.ID {
					p.Type = rowOrPanel.Type
					p.Title = rowOrPanel.Title

					break
				}
			}
		}

		// Create panel model and append to panels
		panels = append(panels, p)
	}

	// Remove xOffset and yOffset from all coordinates of panels
	// and estimate new width scale based on max width
	newScales := scales
	newScales["width"] = math.Round((maxWidth - xOffset) / 24)

	// Estimate Panel coordinates in Grafana column scale
	for ipanel := range panels {
		panels[ipanel].GridPos.X = math.Round((panels[ipanel].GridPos.X - xOffset) / scales["width"])
		panels[ipanel].GridPos.Y = math.Round((panels[ipanel].GridPos.Y - yOffset) / scales["height"])
		panels[ipanel].GridPos.W = math.Round(panels[ipanel].GridPos.W / scales["width"])
		panels[ipanel].GridPos.H = math.Round(panels[ipanel].GridPos.H / scales["height"])
	}

	// Check if we fetched any panels
	if len(panels) == 0 {
		allErrs = errors.Join(err, ErrNoPanels)

		return nil, allErrs
	}

	return panels, allErrs
}

// panelsFromJSON makes panels from dashboard JSON model by uncollapsing and correcting
// grid positions for all row panels when dashboardMode is full.
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
