package dashboard

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/helpers"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Embed the entire directory.
//
//go:embed js
var jsFS embed.FS

// New creates a new instance of the Dashboard struct.
func New(logger log.Logger, conf *config.Config, httpClient *http.Client, chromeInstance chrome.Instance,
	appURL, appVersion string, model *Model, authHeader http.Header,
) (*Dashboard, error) {
	// Parse app URL
	u, err := url.Parse(appURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse app URL: %w", errors.Unwrap(err))
	}

	// Read JS from embedded file
	js, err := jsFS.ReadFile("js/panels.js")
	if err != nil {
		return nil, fmt.Errorf("failed to load JS: %w", err)
	}

	return &Dashboard{
		logger,
		conf,
		httpClient,
		chromeInstance,
		u,
		appVersion,
		string(js),
		model,
		authHeader,
	}, nil
}

// GetData fetches dashboard related data.
func (d *Dashboard) GetData(ctx context.Context) (*Data, error) {
	defer helpers.TimeTrack(time.Now(), "dashboard data", d.logger)

	// Make panels from loading the dashboard in a browser instance
	panels, err := d.panels(ctx)
	if err != nil {
		d.logger.Error("error collecting panels from browser", "error", err)

		return nil, fmt.Errorf("error collecting panels from browser: %w", err)
	}

	return &Data{
		Title:     d.model.Dashboard.Title,
		TimeRange: NewTimeRange(d.model.Dashboard.Variables.Get("from"), d.model.Dashboard.Variables.Get("to")),
		Variables: variablesValues(d.model.Dashboard.Variables),
		Panels:    panels,
	}, err
}

// variablesValues returns current dashboard template variables and their values as
// a string.
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
