package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"golang.org/x/net/context"
)

const SaToken = "saToken"

// Config contains plugin settings
type Config struct {
	AppURL            string `json:"appUrl"`
	Orientation       string `json:"orientation"`
	Layout            string `json:"layout"`
	DashboardMode     string `json:"dashboardMode"`
	TimeZone          string `json:"timeZone"`
	EncodedLogo       string `json:"logo"`
	MaxBrowserWorkers int    `json:"maxBrowserWorkers"`
	MaxRenderWorkers  int    `json:"maxRenderWorkers"`
	RemoteChromeURL   string `json:"remoteChromeURL"`
	IncludePanelIDs   []int
	ExcludePanelIDs   []int

	// HTTP Client
	HTTPClientOptions httpclient.Options

	// Secrets
	Token string
}

// String implements the stringer interface of Config
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
		"Orientation: %s; Layout: %s; Dashboard Mode: %s; Time Zone: %s; Encoded Logo: %s; "+
			"Max Renderer Workers: %d; Max Browser Workers: %d; Remote Chrome Addr: %s; App URL: %s; "+
			"TLS Skip verifiy: %v; Included Panel IDs: %s; Excluded Panel IDs: %s",
		c.Orientation, c.Layout,
		c.DashboardMode, c.TimeZone, encodedLogo, c.MaxRenderWorkers, c.MaxBrowserWorkers,
		c.RemoteChromeURL, appURL,
		c.HTTPClientOptions.TLS.InsecureSkipVerify, includedPanelIDs, excludedPanelIDs,
	)
}

// Load loads the plugin settings from data sent by provisioned config or from Grafana UI
func Load(ctx context.Context, settings backend.AppInstanceSettings) (Config, error) {
	// Always start with a default config so that when the plugin is not provisioned
	// with a config, we will still have "non-null" config to work with
	var config = Config{
		Orientation:       "portrait",
		Layout:            "simple",
		DashboardMode:     "default",
		TimeZone:          "",
		EncodedLogo:       "",
		MaxBrowserWorkers: 6,
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

	if err = json.Unmarshal(settings.JSONData, &config); err != nil {
		return Config{}, err
	}

	// Get default HTTP client options
	config.HTTPClientOptions, err = settings.HTTPClientOptions(ctx)
	if err != nil {
		return Config{}, fmt.Errorf("error in http client options: %w", err)
	}

	if config.HTTPClientOptions.TLS == nil {
		config.HTTPClientOptions.TLS = &httpclient.TLSOptions{}
	}

	// Only allow configuring using GF_* env vars
	// TODO Deprecated: Use tlsSkipVerify instead
	if os.Getenv("GF_REPORTER_PLUGIN_IGNORE_HTTPS_ERRORS") != "" {
		config.HTTPClientOptions.TLS.InsecureSkipVerify = true
	}

	return config, nil
}
