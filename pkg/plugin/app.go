package plugin

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/chrome"
)

const Name = "mahendrapaipuri-dashboardreporter-app"

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
	httpClient *http.Client

	chromeInstance chrome.Instance
}

// NewDashboardReporterApp creates a new example *App instance.
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

	// Get default HTTP client options
	opts, err := settings.HTTPClientOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("error in http client options: %w", err)
	}

	if opts.TLS == nil {
		opts.TLS = &httpclient.TLSOptions{}
	}

	// Only allow configuring using GF_* env vars
	// TODO Deprecated: Use GF_REPORTER_PLUGIN_IGNORE_HTTPS_ERRORS instead
	if os.Getenv("GF_REPORTER_PLUGIN_IGNORE_HTTPS_ERRORS") != "" {
		opts.TLS.InsecureSkipVerify = true
	}

	// Make a new HTTP client
	if app.httpClient, err = httpclient.New(opts); err != nil {
		return nil, fmt.Errorf("error in httpclient new: %w", err)
	}

	// Create a new browser instance
	chromeInstance, err := chrome.NewLocalBrowserInstance(context.Background(), ctxLogger, opts.TLS.InsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to start browser: %w", err)
	}

	// Use the same browser instance for all API requests
	app.chromeInstance = chromeInstance

	return &app, nil
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created.
func (a *App) Dispose() {
	if a.chromeInstance == nil {
		return
	}

	// cleanup old chromium instances
	ctxLogger := log.DefaultLogger.FromContext(context.Background())
	ctxLogger.Info("disposing chromium from old plugin app instance")

	a.chromeInstance.Close()
}

// CheckHealth handles health checks sent from Grafana to the plugin.
func (a *App) CheckHealth(_ context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "ok",
	}, nil
}
