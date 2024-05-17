package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/spf13/afero"
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

// Plugin config settings
type Config struct {
	AppURL           string `json:"appUrl"`
	SkipTLSCheck     bool   `json:"skipTlsCheck"`
	Orientation      string `json:"orientation"`
	Layout           string `json:"layout"`
	DashboardMode    string `json:"dashboardMode"`
	EncodedLogo      string `json:"encodedLogo"`
	MaxRenderWorkers int    `json:"maxRenderWorkers"`
	PersistData      bool   `json:"persistData"`
	DataPath         string
	IncludePanelIDs  []int
	ExcludePanelIDs  []int
	ChromeOptions    []func(*chromedp.ExecAllocator)
}

// String implements the stringer interface of Config
func (c *Config) String() string {
	var encodedLogo string
	if c.EncodedLogo != "" {
		encodedLogo = "[truncated]"
	} else {
		encodedLogo = ""
	}

	var includedPanelIDs, excludedPanelIDs string
	if len(c.IncludePanelIDs) > 0 {
		var panelIDs []string
		for _, id := range c.IncludePanelIDs {
			panelIDs = append(panelIDs, strconv.Itoa(id))
		}
		includedPanelIDs = strings.Join(panelIDs, ",")
	} else {
		includedPanelIDs = "all"
	}
	if len(c.ExcludePanelIDs) > 0 {
		var panelIDs []string
		for _, id := range c.ExcludePanelIDs {
			panelIDs = append(panelIDs, strconv.Itoa(id))
		}
		excludedPanelIDs = strings.Join(panelIDs, ",")
	} else {
		excludedPanelIDs = "none"
	}
	return fmt.Sprintf(
		"Grafana App URL: %s; Skip TLS Check: %t; Grafana Data Path: %s; "+
			"Orientation: %s; Layout: %s; Dashboard Mode: %s; Encoded Logo: %s; Max Renderer Workers: %d; "+
			"Persist Data: %t; Included Panel IDs: %s; Excluded Panel IDs: %s", 
			c.AppURL, c.SkipTLSCheck, c.DataPath, c.Orientation, c.Layout,
		c.DashboardMode, encodedLogo, c.MaxRenderWorkers, c.PersistData, includedPanelIDs,
		excludedPanelIDs,
	)
}

// Plugin secret settings
type Secrets struct {
	cookieHeader string
	token        string
	cookies      []string // Slice of name, value pairs of all cookies applicable to current domain
}

// App is the backend plugin which can respond to api queries.
type App struct {
	backend.CallResourceHandler
	httpClient       *http.Client
	config           *Config
	secrets          *Secrets
	vfs              *afero.BasePathFs
	newGrafanaClient func(client *http.Client, secrets *Secrets, config *Config, variables url.Values) GrafanaClient
	newReport        func(logger log.Logger, grafanaClient GrafanaClient, options *ReportOptions) (Report, error)
}

// NewApp creates a new example *App instance.
func NewApp(ctx context.Context, settings backend.AppInstanceSettings) (instancemgmt.Instance, error) {
	var app App

	// Get context logger for debugging
	ctxLogger := log.DefaultLogger.FromContext(ctx)

	// Use a httpadapter (provided by the SDK) for resource calls. This allows us
	// to use a *http.ServeMux for resource calls, so we can map multiple routes
	// to CallResource without having to implement extra logic.
	mux := http.NewServeMux()
	app.registerRoutes(mux)
	app.CallResourceHandler = httpadapter.New(mux)

	// Get Grafana data path based on path of current executable
	pluginExe, err := os.Executable()
	if err != nil {
		panic(err)
	}

	// Generally this pluginExe should be at install_dir/plugins/mahendrapaipuri-dashboardreporter-app/exe
	// Now we attempt to get install_dir directory which is Grafana data path
	dataPath := filepath.Dir(filepath.Dir(filepath.Dir(pluginExe)))

	// Update plugin settings defaults
	var config = Config{
		DataPath:         dataPath,
		Orientation:      "portrait",
		Layout:           "simple",
		DashboardMode:    "default",
		MaxRenderWorkers: 2,
	}
	if settings.JSONData != nil && string(settings.JSONData) != "null" {
		if err := json.Unmarshal(settings.JSONData, &config); err == nil {
			ctxLogger.Info("provisioned config", "config", config.String())
		} else {
			ctxLogger.Error("failed to load plugin config", "err", err)
		}
	}

	// Fetch token, if configured in SecureJSONData
	var secrets Secrets
	if settings.DecryptedSecureJSONData != nil {
		if saToken, ok := settings.DecryptedSecureJSONData["saToken"]; ok {
			if saToken != "" {
				secrets = Secrets{token: saToken}
				ctxLogger.Info("service account token configured")
			}
		}
	}

	// Make HTTP client
	opts, err := settings.HTTPClientOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("error in http client options: %w", err)
	}

	// If skip verify is set to true configure it for Grafana HTTP client
	if config.SkipTLSCheck {
		opts.TLS = &httpclient.TLSOptions{InsecureSkipVerify: true}
	}

	if app.httpClient, err = httpclient.New(opts); err != nil {
		return nil, fmt.Errorf("error in httpclient new: %w", err)
	}

	// Seems like accessing env vars is not encouraged
	// Ref: https://github.com/grafana/plugin-validator/blob/eb71abbbead549fd7697371b25c226faba19b252/pkg/analysis/passes/coderules/semgrep-rules.yaml#L13-L28
	//
	// appURL set from the env var will always take the highest precedence
	if os.Getenv("GF_APP_URL") != "" {
		config.AppURL = os.Getenv("GF_APP_URL")
		ctxLogger.Debug("using Grafana app URL from environment variable", "GF_APP_URL", config.AppURL)
	}

	if config.AppURL == "" {
		return nil, fmt.Errorf("grafana app URL not found. Please set it in provisioned config")
	}

	// Trim trailing slash in app URL
	config.AppURL = strings.TrimRight(config.AppURL, "/")

	/*
		Create a virtual FS with /var/lib/grafana as base path. In cloud context,
		probably this is the only directory with write permissions. We cannot rely
		on /tmp as containers started in read-only mode will not be able to write to
		/tmp.

		We need a reports directory to save ephermeral files and images, print HTML
		into PDF. We will clean them up after each request and so we will use this
		reports directory to store these files.
	*/
	vfs := afero.NewBasePathFs(afero.NewOsFs(), config.DataPath).(*afero.BasePathFs)

	// Create a reports dir inside this GF_PATHS_DATA folder
	if err = vfs.MkdirAll("reports", 0750); err != nil {
		return nil, fmt.Errorf("failed to create a reports directory in %s: %w", config.DataPath, err)
	}

	// Set chrome options
	config.ChromeOptions = append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		// Seems like this is critical. When it is not turned on there are no errors
		// and plugin will exit without rendering any panels. Not sure why the error
		// handling is failing here. So, add this option as default just to avoid
		// those cases
		//
		// Ref: https://github.com/chromedp/chromedp/issues/492#issuecomment-543223861
		chromedp.Flag("ignore-certificate-errors", "1"),
	)

	/*
		Attempt to use chrome shipped from grafana-image-renderer. If not found,
		use the chromium browser installed on the host.

		We check for the GF_PATHS_DATA env variable and if not found we use default
		/var/lib/grafana. We do a walk dir in $GF_PATHS_DATA/plugins/grafana-image-render
		and try to find `chrome` executable. If we find it, we use it as chrome
		executable for rendering the PDF report.
	*/

	// Chrome executable path
	var chromeExec string

	// Walk through grafana-image-renderer plugin dir to find chrome executable
	err = filepath.Walk(filepath.Join(config.DataPath, "plugins", "grafana-image-renderer"),
		func(path string, info fs.FileInfo, err error) error {
			// prevent panic by handling failure accessing a path
			if err != nil {
				return err
			}
			if !info.IsDir() && info.Name() == "chrome" {
				chromeExec = path
				return nil
			}
			return nil
		})
	if err != nil {
		ctxLogger.Warn("failed to walk through grafana-image-renderer data dir", "err", err)
	}

	// If chrome is found in grafana-image-renderer plugin dir, use it
	if chromeExec != "" {
		ctxLogger.Info("chrome executable provided by grafana-image-renderer will be used", "chrome", chromeExec)
		config.ChromeOptions = append(config.ChromeOptions, chromedp.ExecPath(chromeExec))
	}

	// Make config
	app.config = &config

	// Add secrets to app
	app.secrets = &secrets

	// Set VFS instance to app
	app.vfs = vfs

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
