package report

import (
	"net/http"
	"strings"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/worker"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

type Report struct {
	logger         log.Logger
	conf           *config.Config
	httpClient     *http.Client
	chromeInstance chrome.Instance
	pools          worker.Pools
	dashboard      *dashboard.Dashboard
}

type HTML struct {
	Header string
	Body   string
	Footer string
}

// Data structures used inside HTML template.
type templateData struct {
	Date      string
	Dashboard *dashboard.Data
	Conf      *config.Config
}

// IsGridLayout returns true if layout config is grid.
func (t templateData) IsGridLayout() bool {
	return t.Conf.Layout == "grid"
}

// From returns from time string.
func (t templateData) From() string {
	return t.Dashboard.TimeRange.FromFormatted(t.Conf.Location, t.Conf.TimeFormat)
}

// To returns to time string.
func (t templateData) To() string {
	return t.Dashboard.TimeRange.ToFormatted(t.Conf.Location, t.Conf.TimeFormat)
}

// Logo returns encoded logo.
func (t templateData) Logo() string {
	// If dataURI is passed in format data:image/png;base64,<content> strip header
	parts := strings.Split(t.Conf.EncodedLogo, ",")
	if len(parts) == 2 {
		return parts[1]
	}

	return t.Conf.EncodedLogo
}

// Panels returns dashboard's panels.
func (t templateData) Panels() []dashboard.Panel {
	return t.Dashboard.Panels
}

// Title returns dashboard's title.
func (t templateData) Title() string {
	return t.Dashboard.Title
}

// VariableValues returns dashboards query variables.
func (t templateData) VariableValues() string {
	return t.Dashboard.Variables
}

// Theme returns dashboard's theme.
func (t templateData) Theme() string {
	return t.Conf.Theme
}
