package config

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSettings(t *testing.T) {
	Convey("When creating a new config from minimum JSONData", t, func() {
		const configJSON = `{}`
		configData := json.RawMessage(configJSON)
		config, err := Load(context.Background(), backend.AppInstanceSettings{JSONData: configData})

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "simple")
			So(config.MaxBrowserWorkers, ShouldEqual, 6)
			So(config.MaxRenderWorkers, ShouldEqual, 2)
		})
	})

	Convey("When creating a new config from provisioned JSONData", t, func() {
		const configJSON = `{"layout": "grid"}`
		configData := json.RawMessage(configJSON)
		config, err := Load(context.Background(), backend.AppInstanceSettings{JSONData: configData})

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "grid")
			So(config.MaxBrowserWorkers, ShouldEqual, 6)
			So(config.MaxRenderWorkers, ShouldEqual, 2)
		})
	})

	Convey("When creating a new config from provisioned JSONData and secrets", t, func() {
		const configJSON = `{"layout": "grid"}`
		configData := json.RawMessage(configJSON)
		secretsMap := map[string]string{
			"saToken": "supersecrettoken",
		}
		config, err := Load(context.Background(), backend.AppInstanceSettings{JSONData: configData, DecryptedSecureJSONData: secretsMap})

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "grid")
			So(config.MaxBrowserWorkers, ShouldEqual, 6)
			So(config.MaxRenderWorkers, ShouldEqual, 2)
			So(config.Token, ShouldEqual, "supersecrettoken")
		})
	})
}
