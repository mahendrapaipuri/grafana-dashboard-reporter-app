package plugin

import (
	"encoding/json"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSettings(t *testing.T) {
	Convey("When creating a new config from empty JSONData", t, func() {
		const configJSON = `{}`
		configData := json.RawMessage([]byte(configJSON))
		_, _, err := loadSettings(configData, nil)

		Convey("Config should return error due to missing Grafana App URL", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("When creating a new config from minimum JSONData", t, func() {
		const configJSON = `{"appUrl": "http://localhost:3000"}`
		configData := json.RawMessage([]byte(configJSON))
		config, _, err := loadSettings(configData, nil)

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.AppURL, ShouldEqual, "http://localhost:3000")
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "simple")
			So(config.MaxRenderWorkers, ShouldEqual, 2)
		})
	})

	Convey("When creating a new config from provisioned JSONData", t, func() {
		const configJSON = `{"appUrl": "http://localhost:3000","layout": "grid"}`
		configData := json.RawMessage([]byte(configJSON))
		config, _, err := loadSettings(configData, nil)

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.AppURL, ShouldEqual, "http://localhost:3000")
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "grid")
			So(config.MaxRenderWorkers, ShouldEqual, 2)
		})
	})

	Convey("When creating a new config from provisioned JSONData and secrets", t, func() {
		const configJSON = `{"appUrl": "http://localhost:3000","layout": "grid"}`
		configData := json.RawMessage([]byte(configJSON))
		secretsMap := map[string]string{
			"saToken": "supersecrettoken",
		}
		config, secrets, err := loadSettings(configData, secretsMap)

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.AppURL, ShouldEqual, "http://localhost:3000")
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "grid")
			So(config.MaxRenderWorkers, ShouldEqual, 2)
			So(secrets.token, ShouldEqual, "supersecrettoken")
		})
	})
}

func TestSettingsWithEnvVars(t *testing.T) {
	// Set env vars
	t.Setenv("GF_APP_URL", "https://localhost:3000")
	t.Setenv("GF_REPORTER_PLUGIN_IGNORE_HTTPS_ERRORS", "true")

	Convey("When creating a new config from empty JSONData", t, func() {
		const configJSON = `{}`
		configData := json.RawMessage([]byte(configJSON))
		config, _, err := loadSettings(configData, nil)

		Convey("Config should return default config", func() {
			So(err, ShouldBeNil)
			So(config.AppURL, ShouldEqual, "https://localhost:3000")
			So(config.SkipTLSCheck, ShouldEqual, true)
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "simple")
			So(config.MaxRenderWorkers, ShouldEqual, 2)
		})
	})

	Convey("When creating a new config from JSONData with appUrl", t, func() {
		const configJSON = `{"appUrl": "http://localhost:3000","skipTlsCheck": false}`
		configData := json.RawMessage([]byte(configJSON))
		config, _, err := loadSettings(configData, nil)

		Convey("Config should contain appUrl from env var", func() {
			So(err, ShouldBeNil)
			So(config.AppURL, ShouldEqual, "https://localhost:3000")
			So(config.SkipTLSCheck, ShouldEqual, true)
		})
	})
}
