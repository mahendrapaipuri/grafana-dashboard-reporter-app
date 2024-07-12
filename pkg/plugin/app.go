package plugin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
)

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

// App is the backend plugin which can respond to api queries.
type App struct {
	backend.CallResourceHandler
	httpClient       *http.Client
	config           *Config
	secrets          *Secrets
	ctxCancelFuncs   func()
	newGrafanaClient func(client *http.Client, secrets *Secrets, config *Config, variables url.Values) GrafanaClient
	newReport        func(logger log.Logger, grafanaClient GrafanaClient, options *ReportOptions) (Report, error)
}

// Keep a state of current provisioned config
var currentAppConfig *Config

// currentDashboardReporterAppConfig returns an instance of current app's provisioned config
func currentDashboardReporterAppConfig() *Config {
	return currentAppConfig
}

// NewApp creates a new example *App instance.
func NewDashboardReporterApp(ctx context.Context, settings backend.AppInstanceSettings) (instancemgmt.Instance, error) {
	var app App

	// Get context logger for debugging
	ctxLogger := log.DefaultLogger.FromContext(ctx)
	ctxLogger.Info("new instance of plugin app created")

	// Use a httpadapter (provided by the SDK) for resource calls. This allows us
	// to use a *http.ServeMux for resource calls, so we can map multiple routes
	// to CallResource without having to implement extra logic.
	mux := http.NewServeMux()
	app.registerRoutes(mux)
	app.CallResourceHandler = httpadapter.New(mux)

	// Load plugin settings
	config, secrets, err := loadSettings(settings.JSONData, settings.DecryptedSecureJSONData)
	if err != nil {
		return nil, fmt.Errorf("failed to create dashboard-reporter plugin app instance: %s", err)
	}
	ctxLogger.Info("provisioned config", "config", config.String())
	if secrets.token != "" {
		ctxLogger.Info("service account token configured")
	}

	// Get default HTTP client options
	opts, err := settings.HTTPClientOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("error in http client options: %w", err)
	}

	// If skip verify is set to true configure it for Grafana HTTP client
	if config.SkipTLSCheck {
		opts.TLS = &httpclient.TLSOptions{InsecureSkipVerify: true}
	}

	// Make a new HTTP client
	if app.httpClient, err = httpclient.New(opts); err != nil {
		return nil, fmt.Errorf("error in httpclient new: %w", err)
	}

	// Create a new browser instance
	browserCtx, ctxCancelFuncs, err := newBrowserInstance(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to start browser: %w", err)
	}
	app.ctxCancelFuncs = ctxCancelFuncs

	// Use the same browser instance for all API requests
	config.BrowserContext = browserCtx

	// Set current App's config
	currentAppConfig = config

	// Make config
	app.config = config

	// Add secrets to app
	app.secrets = secrets

	// Add Grafana client and report factory makers
	app.newGrafanaClient = NewGrafanaClient
	app.newReport = NewReport
	return &app, nil
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created.
func (a *App) Dispose() {
	// cleanup old chromium instances
	ctxLogger := log.DefaultLogger.FromContext(context.Background())
	ctxLogger.Info("disposing chromium from old plugin app instance")
	a.ctxCancelFuncs()
}

// CheckHealth handles health checks sent from Grafana to the plugin.
func (a *App) CheckHealth(_ context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "ok",
	}, nil
}
