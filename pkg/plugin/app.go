package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/spf13/afero"
)

const GF_PATHS_DATA = "/var/lib/grafana"
const PLUGIN_NAME = "mahendrapaipuri-dashboardreporter-app"

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
	orientation      string
	layout           string
	dashboardMode    string
	maxRenderWorkers int
	persistData      bool
	vfs              *afero.BasePathFs
}

// App is the backend plugin which can respond to api queries.
type App struct {
	backend.CallResourceHandler
	httpClient       *http.Client
	grafanaAppUrl    string
	config           *Config
	newGrafanaClient func(client *http.Client, grafanaAppURL string, cookie string, variables url.Values, layout string, panels string) GrafanaClient
	newReport        func(logger log.Logger, grafanaClient GrafanaClient, config *ReportConfig) (Report, error)
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
	var grafanaAppUrl string
	var orientation string
	var layout string
	var dashboardMode string
	var maxRenderWorkers int = 2
	var persistData bool = false
	if settings.JSONData != nil {
		if err := json.Unmarshal(settings.JSONData, &data); err == nil {
			if v, exists := data["appUrl"]; exists {
				grafanaAppUrl = strings.TrimRight(v.(string), "/")
			}
			if v, exists := data["orientation"]; exists {
				orientation = v.(string)
			}
			if v, exists := data["layout"]; exists {
				layout = v.(string)
			}
			if v, exists := data["dashboardMode"]; exists {
				dashboardMode = v.(string)
			}
			if v, exists := data["maxRenderWorkers"]; exists {
				maxRenderWorkers = int(v.(float64))
			}
			if v, exists := data["persistData"]; exists {
				persistData = v.(bool)
			}
		}
	}

	// Seems like accessing env vars is not encouraged
	// Ref: https://github.com/grafana/plugin-validator/blob/eb71abbbead549fd7697371b25c226faba19b252/pkg/analysis/passes/coderules/semgrep-rules.yaml#L13-L28
	//
	// If appURL is not found in plugin settings attempt to get it from env var
	if grafanaAppUrl == "" && os.Getenv("GF_APP_URL") != "" {
		grafanaAppUrl = strings.TrimRight(os.Getenv("GF_APP_URL"), "/")
	}

	if grafanaAppUrl == "" {
		return nil, fmt.Errorf("grafana app URL not configured in JSONData")
	}

	/*
		Create a virtual FS with /var/lib/grafana as base path. In cloud context,
		probably this is the only directory with write permissions. We cannot rely
		on /tmp as containers started in read-only mode will not be able to write to
		/tmp.

		We need a reports directory to save ephermeral files and images, print HTML
		into PDF. We will clean them up after each request and so we will use this
		reports directory to store these files.
	*/
	var pluginDir string
	if os.Getenv("GF_PATHS_DATA") != "" {
		pluginDir = os.Getenv("GF_PATHS_DATA")
	} else {
		pluginDir = GF_PATHS_DATA
	}
	vfs := afero.NewBasePathFs(afero.NewOsFs(), pluginDir).(*afero.BasePathFs)

	// Create a reports dir inside this GF_PATHS_DATA folder
	if err = vfs.MkdirAll("reports", 0750); err != nil {
		return nil, fmt.Errorf("failed to create a reports directory in %s: %w", pluginDir, err)
	}

	// Make config
	app.config = &Config{
		orientation:      orientation,
		layout:           layout,
		dashboardMode:    dashboardMode,
		maxRenderWorkers: maxRenderWorkers,
		persistData:      persistData,
		vfs:              vfs,
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
	// cleanup reports dir
	// a.config.vfs.RemoveAll("reports")
}

// CheckHealth handles health checks sent from Grafana to the plugin.
func (a *App) CheckHealth(_ context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "ok",
	}, nil
}
