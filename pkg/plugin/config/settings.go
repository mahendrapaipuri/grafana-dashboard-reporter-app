package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/sethvargo/go-envconfig"
	"golang.org/x/net/context"
)

const SaToken = "saToken"

// Valid setting parameters.
var (
	validThemes       = []string{"light", "dark"}
	validLayouts      = []string{"simple", "grid"}
	validOrientations = []string{"portrait", "landscape"}
	validModes        = []string{"default", "full"}
)

// Config contains plugin settings.
type Config struct {
	AppURL              string            `env:"GF_REPORTER_PLUGIN_APP_URL, overwrite"                json:"appUrl"`
	SkipTLSCheck        bool              `env:"GF_REPORTER_PLUGIN_SKIP_TLS_CHECK, overwrite"         json:"skipTlsCheck"`
	Theme               string            `env:"GF_REPORTER_PLUGIN_REPORT_THEME, overwrite"           json:"theme"`
	Orientation         string            `env:"GF_REPORTER_PLUGIN_REPORT_ORIENTATION, overwrite"     json:"orientation"`
	Layout              string            `env:"GF_REPORTER_PLUGIN_REPORT_LAYOUT, overwrite"          json:"layout"`
	DashboardMode       string            `env:"GF_REPORTER_PLUGIN_REPORT_DASHBOARD_MODE, overwrite"  json:"dashboardMode"`
	TimeZone            string            `env:"GF_REPORTER_PLUGIN_REPORT_TIMEZONE, overwrite"        json:"timeZone"`
	TimeFormat          string            `env:"GF_REPORTER_PLUGIN_REPORT_TIMEFORMAT, overwrite"      json:"timeFormat"`
	EncodedLogo         string            `env:"GF_REPORTER_PLUGIN_REPORT_LOGO, overwrite"            json:"logo"`
	HeaderTemplate      string            `env:"GF_REPORTER_PLUGIN_REPORT_HEADER_TEMPLATE, overwrite" json:"headerTemplate"`
	FooterTemplate      string            `env:"GF_REPORTER_PLUGIN_REPORT_FOOTER_TEMPLATE, overwrite" json:"footerTemplate"`
	MaxBrowserWorkers   int               `env:"GF_REPORTER_PLUGIN_MAX_BROWSER_WORKERS, overwrite"    json:"maxBrowserWorkers"`
	MaxRenderWorkers    int               `env:"GF_REPORTER_PLUGIN_MAX_RENDER_WORKERS, overwrite"     json:"maxRenderWorkers"`
	RemoteChromeURL     string            `env:"GF_REPORTER_PLUGIN_REMOTE_CHROME_URL, overwrite"      json:"remoteChromeUrl"`
	NativeRendering     bool              `env:"GF_REPORTER_PLUGIN_NATIVE_RENDERER, overwrite"        json:"nativeRenderer"`
	CustomHttpHeaders   map[string]string `env:"GF_REPORTER_PLUGIN_CUSTOM_HTTP_HEADERS, overwrite"    json:"customHttpHeaders"`
	AppVersion          string            `json:"appVersion"`
	IncludePanelIDs     []string
	ExcludePanelIDs     []string
	IncludePanelDataIDs []string

	// Time location
	Location *time.Location

	// HTTP Client
	HTTPClientOptions httpclient.Options

	// Secrets
	Token string
}

// Validate checks current settings and sets them to defaults for invalid ones.
func (c *Config) Validate() error {
	// Check theme
	if !slices.Contains(validThemes, c.Theme) {
		return fmt.Errorf("theme: %s must be one of [%s]", c.Theme, strings.Join(validThemes, ","))
	}

	// Check layout
	if !slices.Contains(validLayouts, c.Layout) {
		return fmt.Errorf("layout: %s must be one of [%s]", c.Layout, strings.Join(validLayouts, ","))
	}

	// Check Orientation
	if !slices.Contains(validOrientations, c.Orientation) {
		return fmt.Errorf("orientation: %s must be one of [%s]", c.Orientation, strings.Join(validOrientations, ","))
	}

	// Check Mode
	if !slices.Contains(validModes, c.DashboardMode) {
		return fmt.Errorf("dashboard mode: %s must be one of [%s]", c.DashboardMode, strings.Join(validModes, ","))
	}

	// Set time zone to current server time zone if empty
	if loc, err := time.LoadLocation(c.TimeZone); err != nil || c.TimeZone == "" {
		c.Location = time.Now().Local().Location()
		c.TimeZone = c.Location.String()
	} else {
		c.Location = loc
		c.TimeZone = loc.String()
	}

	// Set time format to time.UnixDate if the provided one is invalid
	t := time.Now().Format(c.TimeFormat)
	if parsedTime, err := time.Parse(c.TimeFormat, t); err != nil || parsedTime.Unix() <= 0 {
		c.TimeFormat = time.UnixDate
	}

	// Verify RemoteChromeURL
	// url.Parse almost allows all the URLs. Need to check Scheme and Host
	if c.RemoteChromeURL != "" {
		if u, err := url.Parse(c.RemoteChromeURL); err != nil {
			return err
		} else {
			if u.Scheme == "" || u.Host == "" {
				return errors.New("remote chrome url is invalid")
			}
		}
	}

	// If AppVersion is empty, set it to 0.0.0
	if c.AppVersion == "" {
		c.AppVersion = "0.0.0"
	}

	return nil
}

// String implements the stringer interface of Config.
func (c *Config) String() string {
	var encodedLogo string
	if c.EncodedLogo != "" {
		encodedLogo = "[truncated]"
	}

	includedPanelIDs := "all"

	if len(c.IncludePanelIDs) > 0 {
		includedPanelIDs = strings.Join(c.IncludePanelIDs, ",")
	}

	excludedPanelIDs := "none"

	if len(c.ExcludePanelIDs) > 0 {
		excludedPanelIDs = strings.Join(c.ExcludePanelIDs, ",")
	}

	includeDataPanelIDs := "none"

	if len(c.IncludePanelDataIDs) > 0 {
		includeDataPanelIDs = strings.Join(c.IncludePanelDataIDs, ",")
	}

	appURL := "unset"
	if c.AppURL != "" {
		appURL = c.AppURL
	}

	return fmt.Sprintf(
		"Theme: %s; Orientation: %s; Layout: %s; Dashboard Mode: %s; "+
			"Time Zone: %s; Time Format: %s; Encoded Logo: %s; "+
			"Max Renderer Workers: %d; Max Browser Workers: %d; Remote Chrome Addr: %s; App URL: %s; "+
			"TLS Skip verify: %v; Included Panel IDs: %s; Excluded Panel IDs: %s Included Data for Panel IDs: %s; "+
			"Native Renderer: %v; Client Timeout: %d",
		c.Theme, c.Orientation, c.Layout, c.DashboardMode, c.TimeZone, c.TimeFormat,
		encodedLogo, c.MaxRenderWorkers, c.MaxBrowserWorkers, c.RemoteChromeURL, appURL,
		c.SkipTLSCheck, includedPanelIDs, excludedPanelIDs, includeDataPanelIDs, c.NativeRendering,
		int(c.HTTPClientOptions.Timeouts.Timeout.Seconds()),
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
		TimeFormat:        "",
		EncodedLogo:       "",
		HeaderTemplate:    "",
		FooterTemplate:    "",
		MaxBrowserWorkers: 2,
		MaxRenderWorkers:  2,
		HTTPClientOptions: httpclient.Options{
			TLS: &httpclient.TLSOptions{
				InsecureSkipVerify: false,
			},
		},
	}

	var err error

	// Fetch token, if configured in SecureJSONData
	if settings.DecryptedSecureJSONData != nil {
		if saToken, ok := settings.DecryptedSecureJSONData[SaToken]; ok && saToken != "" {
			config.Token = saToken
		}
	}

	// Update plugin settings defaults
	if settings.JSONData != nil && string(settings.JSONData) != "null" {
		if err = json.Unmarshal(settings.JSONData, &config); err != nil { //nolint:musttag
			return Config{}, err
		}
	}

	// Override provisioned config from env vars, if set
	if err := envconfig.Process(ctx, &config); err != nil {
		return Config{}, fmt.Errorf("error in reading config env vars: %w", err)
	}

	// Initialize CustomHttpHeaders if nil
	if config.CustomHttpHeaders == nil {
		config.CustomHttpHeaders = make(map[string]string)
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("error in config settings: %w", err)
	}

	// Get default HTTP client options
	if config.HTTPClientOptions, err = settings.HTTPClientOptions(ctx); err != nil {
		return Config{}, fmt.Errorf("error in http client options: %w", err)
	}

	config.HTTPClientOptions.TLS = &httpclient.TLSOptions{InsecureSkipVerify: config.SkipTLSCheck}

	return config, nil
}
