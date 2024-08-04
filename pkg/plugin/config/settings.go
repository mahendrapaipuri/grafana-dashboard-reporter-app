package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const SaToken = "saToken"

// Config contains plugin settings
type Config struct {
	AppURL            string `json:"appURL"`
	TLSSkipVerify     bool   `json:"tlsSkipVerify"`
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
		c.TLSSkipVerify, includedPanelIDs, excludedPanelIDs,
	)
}

// Load loads the plugin settings from data sent by provisioned config or from Grafana UI
func Load(data json.RawMessage, secureData map[string]string) (*Config, error) {
	// Always start with a default config so that when the plugin is not provisioned
	// with a config, we will still have "non-null" config to work with
	var config = &Config{
		Orientation:       "portrait",
		Layout:            "simple",
		DashboardMode:     "default",
		TimeZone:          "",
		EncodedLogo:       "",
		MaxBrowserWorkers: 6,
		MaxRenderWorkers:  2,
	}

	// Fetch token, if configured in SecureJSONData
	if secureData != nil {
		if saToken, ok := secureData[SaToken]; ok && saToken != "" {
			config.Token = saToken
		}
	}

	// Update plugin settings defaults
	if data == nil || string(data) == "null" {
		return config, nil
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}
