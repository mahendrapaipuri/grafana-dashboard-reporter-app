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
type Row struct {
	Id        int     `json:"id"`
	Collapsed bool    `json:"collapsed"`
	Title     string  `json:"title"`
	Panels    []Panel `json:"panels"`
}

// If row is visible
func (r Row) IsVisible() bool {
	return r.Collapsed
}

// Dashboard represents a Grafana dashboard
// This is both used to unmarshal the dashbaord JSON into
// and then enriched (sanitize fields for TeX consumption and add VarialbeValues)
type Dashboard struct {
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	VariableValues string  // Not present in the Grafana JSON structure. Enriched data passed used by the Tex templating
	Rows           []Row   `json:"rows"`
	Panels         []Panel `json:"panels"`
}

// Get dashboard variables
func getVariablesValues(queryParams url.Values) string {
	values := []string{}
	for n, v := range queryParams {
		values = append(values, fmt.Sprintf("%s=%s", n, strings.Join(v, ",")))
	}
	return strings.Join(values, "; ")
}

// NewDashboard creates Dashboard from Grafana's internal JSON dashboard definition
func NewDashboard(dashJSON []byte, queryParams url.Values) Dashboard {
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
		for _, p := range dashboard.Panels {
			if p.Type == "row" {
				continue
			}
			filteredPanels = append(filteredPanels, p)
		}
		dashboard.Panels = filteredPanels
		dashboard.VariableValues = getVariablesValues(queryParams)
		return dashboard
	}
}
