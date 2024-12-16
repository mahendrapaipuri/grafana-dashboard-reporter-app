package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/worker"
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

// New creates a new instance of the Dashboard struct.
func New(logger log.Logger, conf *config.Config, httpClient *http.Client, chromeInstance chrome.Instance,
	pools worker.Pools, appURL, appVersion string, model *Model, authHeader http.Header,
) *Dashboard {
	return &Dashboard{
		logger,
		conf,
		httpClient,
		chromeInstance,
		pools,
		appURL,
		appVersion,
		model,
		authHeader,
	}
}

// GetData fetches dashboard related data.
func (d *Dashboard) GetData(ctx context.Context) (*Data, error) {
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
