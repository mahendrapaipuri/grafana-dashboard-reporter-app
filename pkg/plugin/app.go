package plugin

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/worker"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/mahendrapaipuri/authlib/authz"
)

type customHeaderTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *customHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for name, value := range t.headers {
		req.Header.Set(name, value)
	}

	return t.base.RoundTrip(req)
}

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

	grafanaSemVer string
	httpClient    *http.Client

	authzClient authz.EnforcementClient
	mx          sync.Mutex

	saToken string
	conf    config.Config

	workerPools    worker.Pools
	chromeInstance chrome.Instance
	ctxLogger      log.Logger
}

// NewDashboardReporterApp creates a new example *App instance.
func NewDashboardReporterApp(ctx context.Context, settings backend.AppInstanceSettings) (instancemgmt.Instance, error) {
	var (
		app App
		err error
	)

	// Get context logger for debugging
	app.ctxLogger = log.DefaultLogger.FromContext(ctx)
	app.ctxLogger.Info("new instance of plugin app created")

	// Use a httpadapter (provided by the SDK) for resource calls. This allows us
	// to use a *http.ServeMux for resource calls, so we can map multiple routes
	// to CallResource without having to implement extra logic.
	mux := http.NewServeMux()
	app.registerRoutes(mux)
	app.CallResourceHandler = httpadapter.New(mux)

	app.conf, err = config.Load(ctx, settings)
	if err != nil {
		app.ctxLogger.Error("error loading config", "err", err)

		return nil, fmt.Errorf("error loading config: %w", err)
	}

	app.ctxLogger.Info("starting plugin with initial config: " + app.conf.String())

	// Get current Grafana version
	app.grafanaSemVer = "v" + backend.UserAgentFromContext(ctx).GrafanaVersion()

	if app.grafanaSemVer == "v0.0.0" && app.conf.AppVersion != "0.0.0" {
		app.grafanaSemVer = "v" + app.conf.AppVersion

		app.ctxLogger.Debug("got grafana version from plugin settings", "version", app.grafanaSemVer)
	} else {
		app.ctxLogger.Debug("got grafana version from backend user agent", "version", app.grafanaSemVer)
	}

	// Make a new HTTP client
	if app.httpClient, err = httpclient.New(app.conf.HTTPClientOptions); err != nil {
		return nil, fmt.Errorf("error in httpclient new: %w", err)
	}

	// Add custom headers to the HTTP client if configured
	if len(app.conf.CustomHttpHeaders) > 0 {
		app.httpClient.Transport = &customHeaderTransport{
			base:    app.httpClient.Transport,
			headers: app.conf.CustomHttpHeaders,
		}
	}

	// Create a new browser instance
	var chromeInstance chrome.Instance

	switch app.conf.RemoteChromeURL {
	case "":
		chromeInstance, err = chrome.NewLocalBrowserInstance(
			context.Background(),
			app.ctxLogger,
			app.conf.HTTPClientOptions.TLS.InsecureSkipVerify,
		)
	default:
		chromeInstance, err = chrome.NewRemoteBrowserInstance(
			context.Background(),
			app.ctxLogger,
			app.conf.RemoteChromeURL,
		)
	}

	if err != nil {
		app.ctxLogger.Error("failed to start browser", "err", err)

		return nil, fmt.Errorf("failed to start browser: %w", err)
	}

	// Use the same browser instance for all API requests
	app.chromeInstance = chromeInstance

	// Span Worker Pool across multiple instances
	// Seems like context passed by App instance is closing channel at the end of
	// request which I dont understand.
	// So, discard context from the App. Always use background context and as we are
	// safely disposing both workers and chrome instances in dispose() method, we are
	// sure that there wont be any leaks.
	app.workerPools = worker.Pools{
		worker.Browser:  worker.New(context.Background(), app.conf.MaxBrowserWorkers),
		worker.Renderer: worker.New(context.Background(), app.conf.MaxRenderWorkers),
	}

	return &app, nil
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created.
func (app *App) Dispose() {
	// Clean up idle connections
	app.httpClient.CloseIdleConnections()

	if app.workerPools != nil {
		for _, pool := range app.workerPools {
			pool.Done()
		}
	}

	if app.chromeInstance == nil {
		return
	}

	// cleanup old chromium instances
	app.ctxLogger.Info("disposing chromium from old plugin app instance")
	app.chromeInstance.Close(app.ctxLogger)
}

// CheckHealth handles health checks sent from Grafana to the plugin.
func (app *App) CheckHealth(_ context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "ok",
	}, nil
}
