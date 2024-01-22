package plugin

import (
	"encoding/json"
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
	if p.Type == t.string() {
		return true
	}
	return false
}

// Row represents a container for Panels
type Row struct {
	Id        int
	Collapsed bool
	Title     string
	Panels    []Panel
}

// If row is visible
func (r Row) IsVisible() bool {
	return r.Collapsed
}

// Dashboard represents a Grafana dashboard
// This is both used to unmarshal the dashbaord JSON into
// and then enriched (sanitize fields for TeX consumption and add VarialbeValues)
type Dashboard struct {
	Title          string
	Description    string
	VariableValues string // Not present in the Grafana JSON structure. Enriched data passed used by the Tex templating
	Rows           []Row
	Panels         []Panel
}

type dashContainer struct {
	Dashboard Dashboard
}

// Populate panels from dashboard JSON model
func populatePanelsFromJSON(dash Dashboard, dc dashContainer) Dashboard {
	for _, p := range dc.Dashboard.Panels {
		if p.Type == "row" {
			continue
		}
		p.Title = sanitizeLaTeXInput(p.Title)
		dash.Panels = append(dash.Panels, p)
	}
	return dash
}

// Get dashboard variables
func getVariablesValues(variables url.Values) string {
	values := []string{}
	for _, v := range variables {
		values = append(values, strings.Join(v, ", "))
	}
	return strings.Join(values, ", ")
}

// Escape LaTeX characters
func sanitizeLaTeXInput(input string) string {
	input = strings.Replace(input, "\\", "\\textbackslash ", -1)
	input = strings.Replace(input, "&", "\\&", -1)
	input = strings.Replace(input, "%", "\\%", -1)
	input = strings.Replace(input, "$", "\\$", -1)
	input = strings.Replace(input, "#", "\\#", -1)
	input = strings.Replace(input, "_", "\\_", -1)
	input = strings.Replace(input, "{", "\\{", -1)
	input = strings.Replace(input, "}", "\\}", -1)
	input = strings.Replace(input, "~", "\\textasciitilde ", -1)
	input = strings.Replace(input, "^", "\\textasciicircum ", -1)
	return input
}

// NewDashboard creates Dashboard from Grafana's internal JSON dashboard definition
func NewDashboard(dashJSON []byte, variables url.Values) Dashboard {
	var dash dashContainer
	err := json.Unmarshal(dashJSON, &dash)
	if err != nil {
		panic(err)
	}
	d := dash.NewDashboard(variables)
	return d
}

// Create a new dashboard
func (dc dashContainer) NewDashboard(variables url.Values) Dashboard {
	var dash Dashboard
	dash.Title = sanitizeLaTeXInput(dc.Dashboard.Title)
	dash.Description = sanitizeLaTeXInput(dc.Dashboard.Description)
	dash.VariableValues = sanitizeLaTeXInput(getVariablesValues(variables))
	return populatePanelsFromJSON(dash, dc)
}
