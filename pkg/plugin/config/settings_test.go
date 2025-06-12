package config

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSettings(t *testing.T) {
	Convey("When creating a new config from minimum JSONData", t, func() {
		const configJSON = `{}`
		configData := json.RawMessage(configJSON)
		config, err := Load(t.Context(), backend.AppInstanceSettings{JSONData: configData})

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "simple")
			So(config.MaxBrowserWorkers, ShouldEqual, 2)
			So(config.MaxRenderWorkers, ShouldEqual, 2)
		})
	})

	Convey("When creating a new config from provisioned JSONData", t, func() {
		const configJSON = `{"layout": "grid"}`
		configData := json.RawMessage(configJSON)
		config, err := Load(t.Context(), backend.AppInstanceSettings{JSONData: configData})

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "grid")
			So(config.MaxBrowserWorkers, ShouldEqual, 2)
			So(config.MaxRenderWorkers, ShouldEqual, 2)
		})
	})

	Convey("When creating a new config from provisioned JSONData and secrets", t, func() {
		const configJSON = `{"layout": "grid"}`
		configData := json.RawMessage(configJSON)
		secretsMap := map[string]string{
			"saToken": "supersecrettoken",
		}
		config, err := Load(
			t.Context(),
			backend.AppInstanceSettings{JSONData: configData, DecryptedSecureJSONData: secretsMap},
		)

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "grid")
			So(config.MaxBrowserWorkers, ShouldEqual, 2)
			So(config.MaxRenderWorkers, ShouldEqual, 2)
			So(config.Token, ShouldEqual, "supersecrettoken")
		})
	})
}

func TestSettingsWithCustomHeaders(t *testing.T) {
	Convey("When creating a new config with custom HTTP headers", t, func() {
		const configJSON = `{"customHttpHeaders": {"X-Custom-Header": "test-value", "Authorization": "Bearer token123"}}`
		configData := json.RawMessage(configJSON)
		config, err := Load(t.Context(), backend.AppInstanceSettings{JSONData: configData})

		Convey("Config should contain custom HTTP headers", func() {
			So(err, ShouldBeNil)
			So(config.CustomHttpHeaders, ShouldNotBeNil)
			So(len(config.CustomHttpHeaders), ShouldEqual, 2)
			So(config.CustomHttpHeaders["X-Custom-Header"], ShouldEqual, "test-value")
			So(config.CustomHttpHeaders["Authorization"], ShouldEqual, "Bearer token123")
		})
	})

	Convey("When creating a new config with empty custom HTTP headers", t, func() {
		const configJSON = `{}`
		configData := json.RawMessage(configJSON)
		config, err := Load(t.Context(), backend.AppInstanceSettings{JSONData: configData})

		Convey("Config should have empty custom HTTP headers map", func() {
			So(err, ShouldBeNil)
			So(config.CustomHttpHeaders, ShouldNotBeNil)
			So(len(config.CustomHttpHeaders), ShouldEqual, 0)
		})
	})
}

func TestSettingsUsingEnvVars(t *testing.T) {
	// Setup env vars
	t.Setenv("GF_REPORTER_PLUGIN_APP_URL", "https://localhost:3000")
	t.Setenv("GF_REPORTER_PLUGIN_SKIP_TLS_CHECK", "true")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_THEME", "light")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_ORIENTATION", "landscape")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_LAYOUT", "grid")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_DASHBOARD_MODE", "full")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_TIMEZONE", "America/New_York")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_LOGO", "encodedlogo")
	t.Setenv("GF_REPORTER_PLUGIN_REMOTE_CHROME_URL", "ws://localhost:5333")

	Convey("When creating a new config from only env vars", t, func() {
		const configJSON = `{}`
		configData := json.RawMessage(configJSON)
		config, err := Load(t.Context(), backend.AppInstanceSettings{JSONData: configData})

		Convey("Config should contain config from env vars", func() {
			So(err, ShouldBeNil)
			So(config.AppURL, ShouldEqual, "https://localhost:3000")
			So(config.SkipTLSCheck, ShouldEqual, true)
			So(config.Theme, ShouldEqual, "light")
			So(config.Orientation, ShouldEqual, "landscape")
			So(config.Layout, ShouldEqual, "grid")
			So(config.DashboardMode, ShouldEqual, "full")
			So(config.TimeZone, ShouldEqual, "America/New_York")
			So(config.EncodedLogo, ShouldEqual, "encodedlogo")
			So(config.MaxBrowserWorkers, ShouldEqual, 2)
			So(config.MaxRenderWorkers, ShouldEqual, 2)
			So(config.RemoteChromeURL, ShouldEqual, "ws://localhost:5333")
		})
	})
}

func TestSettingsUsingConfigAndEnvVars(t *testing.T) {
	// Setup env vars
	t.Setenv("GF_REPORTER_PLUGIN_SKIP_TLS_CHECK", "true")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_THEME", "light")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_ORIENTATION", "landscape")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_LAYOUT", "grid")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_TIMEZONE", "America/New_York")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_LOGO", "encodedlogo")
	t.Setenv("GF_REPORTER_PLUGIN_REMOTE_CHROME_URL", "ws://localhost:5333")

	Convey("When creating a new config from file and env vars", t, func() {
		const configJSON = `{"appUrl": "https://localhost:3000","dashboardMode": "full"}`
		configData := json.RawMessage(configJSON)
		config, err := Load(t.Context(), backend.AppInstanceSettings{JSONData: configData})

		Convey("Config should contain config from file and env vars", func() {
			So(err, ShouldBeNil)
			So(config.AppURL, ShouldEqual, "https://localhost:3000")
			So(config.SkipTLSCheck, ShouldEqual, true)
			So(config.Theme, ShouldEqual, "light")
			So(config.Orientation, ShouldEqual, "landscape")
			So(config.Layout, ShouldEqual, "grid")
			So(config.DashboardMode, ShouldEqual, "full")
			So(config.TimeZone, ShouldEqual, "America/New_York")
			So(config.EncodedLogo, ShouldEqual, "encodedlogo")
			So(config.MaxBrowserWorkers, ShouldEqual, 2)
			So(config.MaxRenderWorkers, ShouldEqual, 2)
			So(config.RemoteChromeURL, ShouldEqual, "ws://localhost:5333")
		})
	})
}

func TestSettingsUsingConfigAndOverridingEnvVars(t *testing.T) {
	// Setup env vars
	t.Setenv("GF_REPORTER_PLUGIN_APP_URL", "https://example.grafana.com")
	t.Setenv("GF_REPORTER_PLUGIN_SKIP_TLS_CHECK", "true")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_THEME", "light")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_ORIENTATION", "landscape")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_LAYOUT", "grid")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_TIMEZONE", "America/New_York")
	t.Setenv("GF_REPORTER_PLUGIN_REPORT_LOGO", "encodedlogo")
	t.Setenv("GF_REPORTER_PLUGIN_REMOTE_CHROME_URL", "ws://localhost:5333")

	Convey("When creating a new config from file and overriding them from env vars", t, func() {
		const configJSON = `{"appUrl": "https://localhost:3000","theme": "dark", "dashboardMode": "full"}`
		configData := json.RawMessage(configJSON)
		config, err := Load(t.Context(), backend.AppInstanceSettings{JSONData: configData})

		Convey("Config should contain config overridden from env vars", func() {
			So(err, ShouldBeNil)
			So(config.AppURL, ShouldEqual, "https://example.grafana.com")
			So(config.SkipTLSCheck, ShouldEqual, true)
			So(config.Theme, ShouldEqual, "light")
			So(config.Orientation, ShouldEqual, "landscape")
			So(config.Layout, ShouldEqual, "grid")
			So(config.DashboardMode, ShouldEqual, "full")
			So(config.TimeZone, ShouldEqual, "America/New_York")
			So(config.EncodedLogo, ShouldEqual, "encodedlogo")
			So(config.MaxBrowserWorkers, ShouldEqual, 2)
			So(config.MaxRenderWorkers, ShouldEqual, 2)
			So(config.RemoteChromeURL, ShouldEqual, "ws://localhost:5333")
		})
	})
}
