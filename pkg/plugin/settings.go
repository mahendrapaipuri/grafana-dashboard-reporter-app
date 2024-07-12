package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config contains plugin settings
type Config struct {
	AppURL           string `json:"appUrl"`
	SkipTLSCheck     bool   `json:"skipTlsCheck"`
	Orientation      string `json:"orientation"`
	Layout           string `json:"layout"`
	DashboardMode    string `json:"dashboardMode"`
	TimeZone         string `json:"timeZone"`
	EncodedLogo      string `json:"logo"`
	MaxRenderWorkers int    `json:"maxRenderWorkers"`
	IncludePanelIDs  []int
	ExcludePanelIDs  []int
	BrowserContext   context.Context
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
		"Grafana App URL: %s; Skip TLS Check: %t; "+
			"Orientation: %s; Layout: %s; Dashboard Mode: %s; Time Zone: %s; Encoded Logo: %s; "+
			"Max Renderer Workers: %d; "+
			"Included Panel IDs: %s; Excluded Panel IDs: %s",
		c.AppURL, c.SkipTLSCheck, c.Orientation, c.Layout,
		c.DashboardMode, c.TimeZone, encodedLogo, c.MaxRenderWorkers,
		includedPanelIDs, excludedPanelIDs,
	)
}

// Secrets contain cookies and tokens
type Secrets struct {
	cookieHeader string
	token        string
	cookies      []string // Slice of name, value pairs of all cookies applicable to current domain
}

// loadSettings loads the plugin settings from data sent by provisioned config or from
// Grafana UI
func loadSettings(data json.RawMessage, secureData map[string]string) (*Config, *Secrets, error) {
	// Always start with a default config so that when the plugin is not provisioned
	// with a config, we will still have "non-null" config to work with
	var config = Config{
		Orientation:      "portrait",
		Layout:           "simple",
		DashboardMode:    "default",
		TimeZone:         "",
		EncodedLogo:      "",
		MaxRenderWorkers: 2,
	}
	// Update plugin settings defaults
	if data != nil && string(data) != "null" {
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, nil, err
		}
	}

	// Read AppURL and SkipTLSCheck from env vars and if found, override them
	if os.Getenv("GF_APP_URL") != "" {
		config.AppURL = os.Getenv("GF_APP_URL")
	}
	// grafana-image-renderer uses IGNORE_HTTPS_ERRORS to skip TLS check and we
	// leverage it here
	if os.Getenv("IGNORE_HTTPS_ERRORS") != "" || os.Getenv("GF_REPORTER_PLUGIN_IGNORE_HTTPS_ERRORS") != "" {
		config.SkipTLSCheck = true
	}

	// If AppURL is still not found return error
	if config.AppURL == "" {
		return nil, nil, fmt.Errorf("grafana app URL not found. Please set it in provisioned config")
	}

	// Trim trailing slash in app URL
	config.AppURL = strings.TrimRight(config.AppURL, "/")

	// Fetch token, if configured in SecureJSONData
	var secrets Secrets
	if secureData != nil {
		if saToken, ok := secureData["saToken"]; ok {
			if saToken != "" {
				secrets = Secrets{token: saToken}
			}
		}
	}
	return &config, &secrets, nil
}
