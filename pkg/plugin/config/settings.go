package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/sethvargo/go-envconfig"
	"golang.org/x/net/context"
)

const SaToken = "saToken"

// Config contains plugin settings.
type Config struct {
	AppURL            string `env:"GF_REPORTER_PLUGIN_APP_URL, overwrite"               json:"appUrl"`
	SkipTLSCheck      bool   `env:"GF_REPORTER_PLUGIN_SKIP_TLS_CHECK, overwrite"        json:"skipTlsCheck"`
	Theme             string `env:"GF_REPORTER_PLUGIN_REPORT_THEME, overwrite"          json:"theme"`
	Orientation       string `env:"GF_REPORTER_PLUGIN_REPORT_ORIENTATION, overwrite"    json:"orientation"`
	Layout            string `env:"GF_REPORTER_PLUGIN_REPORT_LAYOUT, overwrite"         json:"layout"`
	DashboardMode     string `env:"GF_REPORTER_PLUGIN_REPORT_DASHBOARD_MODE, overwrite" json:"dashboardMode"`
	TimeZone          string `env:"GF_REPORTER_PLUGIN_REPORT_TIMEZONE, overwrite"       json:"timeZone"`
	EncodedLogo       string `env:"GF_REPORTER_PLUGIN_REPORT_LOGO, overwrite"           json:"logo"`
	MaxBrowserWorkers int    `env:"GF_REPORTER_PLUGIN_MAX_BROWSER_WORKERS, overwrite"   json:"maxBrowserWorkers"`
	MaxRenderWorkers  int    `env:"GF_REPORTER_PLUGIN_MAX_RENDER_WORKERS, overwrite"    json:"maxRenderWorkers"`
	RemoteChromeURL   string `env:"GF_REPORTER_PLUGIN_REMOTE_CHROME_URL, overwrite"     json:"remoteChromeUrl"`
	IncludePanelIDs   []int
	ExcludePanelIDs   []int

	// HTTP Client
	HTTPClientOptions httpclient.Options

	// Secrets
	Token string
}

// String implements the stringer interface of Config.
func (c *Config) String() string {
	var encodedLogo string
	if c.EncodedLogo != "" {
		encodedLogo = "[truncated]"
	}

	includedPanelIDs := "all"

	if len(c.IncludePanelIDs) > 0 {
		panelIDs := make([]string, len(c.IncludePanelIDs))
		for index, id := range c.IncludePanelIDs {
			panelIDs[index] = strconv.Itoa(id)
		}

		includedPanelIDs = strings.Join(panelIDs, ",")
	}

	excludedPanelIDs := "none"

	if len(c.ExcludePanelIDs) > 0 {
		panelIDs := make([]string, len(c.ExcludePanelIDs))
		for index, id := range c.ExcludePanelIDs {
			panelIDs[index] = strconv.Itoa(id)
		}

		excludedPanelIDs = strings.Join(panelIDs, ",")
	}

	appURL := "unset"
	if c.AppURL != "" {
		appURL = c.AppURL
	}

	return fmt.Sprintf(
		"Theme: %s; Orientation: %s; Layout: %s; Dashboard Mode: %s; Time Zone: %s; Encoded Logo: %s; "+
			"Max Renderer Workers: %d; Max Browser Workers: %d; Remote Chrome Addr: %s; App URL: %s; "+
			"TLS Skip verifiy: %v; Included Panel IDs: %s; Excluded Panel IDs: %s",
		c.Theme, c.Orientation, c.Layout,
		c.DashboardMode, c.TimeZone, encodedLogo, c.MaxRenderWorkers, c.MaxBrowserWorkers,
		c.RemoteChromeURL, appURL,
		c.SkipTLSCheck, includedPanelIDs, excludedPanelIDs,
	)
}

// Load loads the plugin settings from data sent by provisioned config or from Grafana UI.
func Load(ctx context.Context, settings backend.AppInstanceSettings) (Config, error) {
	// Always start with a default config so that when the plugin is not provisioned
	// with a config, we will still have "non-null" config to work with
	config := Config{
		Theme:             "light",
		Orientation:       "portrait",
		Layout:            "simple",
		DashboardMode:     "default",
		TimeZone:          "",
		EncodedLogo:       "",
		MaxBrowserWorkers: 2,
		MaxRenderWorkers:  2,
		HTTPClientOptions: httpclient.Options{
			TLS: &httpclient.TLSOptions{
				InsecureSkipVerify: false,
			},
		},
	}

	// Fetch token, if configured in SecureJSONData
	if settings.DecryptedSecureJSONData != nil {
		if saToken, ok := settings.DecryptedSecureJSONData[SaToken]; ok && saToken != "" {
			config.Token = saToken
		}
	}

	// Update plugin settings defaults
	if settings.JSONData == nil || string(settings.JSONData) == "null" {
		return config, nil
	}

	var err error

	if err = json.Unmarshal(settings.JSONData, &config); err != nil { //nolint:musttag
		return Config{}, err
	}

	// Override provisioned config from env vars, if set
	if err := envconfig.Process(ctx, &config); err != nil {
		return Config{}, fmt.Errorf("error in reading config env vars: %w", err)
	}

	// Get default HTTP client options
	if config.HTTPClientOptions, err = settings.HTTPClientOptions(ctx); err != nil {
		return Config{}, fmt.Errorf("error in http client options: %w", err)
	}

	config.HTTPClientOptions.TLS = &httpclient.TLSOptions{InsecureSkipVerify: config.SkipTLSCheck}

	return config, nil
}
