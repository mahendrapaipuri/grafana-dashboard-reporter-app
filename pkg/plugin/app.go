package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
)

// Make sure App implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. Plugin should not implement all these interfaces - only those which are
// required for a particular task.
var (
	_ backend.CallResourceHandler   = (*App)(nil)
	_ instancemgmt.InstanceDisposer = (*App)(nil)
	_ backend.CheckHealthHandler    = (*App)(nil)
)

// Plugin config settings
type Config struct {
	useGridLayout    bool
	texTemplate      string
	maxRenderWorkers int
	stagingDir       string
}

// App is the backend plugin which can respond to api queries.
type App struct {
	backend.CallResourceHandler
	httpClient    *http.Client
	grafanaAppUrl string
	config        *Config
	newGrafanaClient func(client *http.Client, grafanaAppURL string, cookie string, variables url.Values, useGridLayout bool) GrafanaClient
	newReport func(logger log.Logger, grafanaClient GrafanaClient, config *ReportConfig) Report
}

// NewApp creates a new example *App instance.
func NewApp(ctx context.Context, settings backend.AppInstanceSettings) (instancemgmt.Instance, error) {
	var app App

	// Get context logger for debugging
	// ctxLogger := log.DefaultLogger.FromContext(ctx)

	// Use a httpadapter (provided by the SDK) for resource calls. This allows us
	// to use a *http.ServeMux for resource calls, so we can map multiple routes
	// to CallResource without having to implement extra logic.
	mux := http.NewServeMux()
	app.registerRoutes(mux)
	app.CallResourceHandler = httpadapter.New(mux)

	opts, err := settings.HTTPClientOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("error in http client options: %w", err)
	}

	cl, err := httpclient.New(opts)
	if err != nil {
		return nil, fmt.Errorf("error in httpclient new: %w", err)
	}
	app.httpClient = cl

	// Get Grafana App URL from plugin settings
	var data map[string]interface{}
	var grafanaAppUrl, texTemplate string
	var useGridLayout bool
	var maxRenderWorkers int = 2
	if settings.JSONData != nil {
		if err := json.Unmarshal(settings.JSONData, &data); err == nil {
			if v, exists := data["appUrl"]; exists {
				grafanaAppUrl = strings.TrimRight(v.(string), "/")
			}
			if v, exists := data["texTemplate"]; exists {
				texTemplate = v.(string)
			}
			if v, exists := data["useGridLayout"]; exists {
				useGridLayout = v.(bool)
			}
			if v, exists := data["maxRenderWorkers"]; exists {
				maxRenderWorkers = int(v.(float64))
			}
		}
	}

	// If appURL is not found in plugin settings attempt to get it from env var
	if grafanaAppUrl == "" && os.Getenv("GF_APP_URL") != "" {
		grafanaAppUrl = strings.TrimRight(os.Getenv("GF_APP_URL"), "/")
	}

	if grafanaAppUrl == "" {
		return nil, fmt.Errorf("Grafana app URL not configured in JsonData or GF_APP_URL environment variable")
	}

	// Make staging directory
	stagingDir, err := filepath.Abs("staging")
	if err != nil {
		return nil, fmt.Errorf("failed to get staging directory")
	}

	// Make config
	app.config = &Config{
		texTemplate:      texTemplate,
		useGridLayout:    useGridLayout,
		maxRenderWorkers: maxRenderWorkers,
		stagingDir:       stagingDir,
	}

	// Add Grafana App URL
	app.grafanaAppUrl = grafanaAppUrl

	// Add Grafana client and report factory makers
	app.newGrafanaClient = NewGrafanaClient
	app.newReport = NewReport
	return &app, nil
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created.
func (a *App) Dispose() {
	// cleanup staging dir
	os.RemoveAll(a.config.stagingDir)
}

// CheckHealth handles health checks sent from Grafana to the plugin.
func (a *App) CheckHealth(_ context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "ok",
	}, nil
}
