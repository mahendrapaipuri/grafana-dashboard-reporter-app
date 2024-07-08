package config

import (
	"encoding/json"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSettings(t *testing.T) {
	Convey("When creating a new config from minimum JSONData", t, func() {
		const configJSON = `{}`
		configData := json.RawMessage(configJSON)
		config, err := Load(configData, nil)

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "simple")
			So(config.MaxRenderWorkers, ShouldEqual, 2)
		})
	})

	Convey("When creating a new config from provisioned JSONData", t, func() {
		const configJSON = `{"layout": "grid"}`
		configData := json.RawMessage(configJSON)
		config, err := Load(configData, nil)

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "grid")
			So(config.MaxRenderWorkers, ShouldEqual, 2)
		})
	})

	Convey("When creating a new config from provisioned JSONData and secrets", t, func() {
		const configJSON = `{"layout": "grid"}`
		configData := json.RawMessage(configJSON)
		secretsMap := map[string]string{
			"saToken": "supersecrettoken",
		}
		config, err := Load(configData, secretsMap)

		Convey("Config should contain default config", func() {
			So(err, ShouldBeNil)
			So(config.Orientation, ShouldEqual, "portrait")
			So(config.Layout, ShouldEqual, "grid")
			So(config.MaxRenderWorkers, ShouldEqual, 2)
			So(config.Token, ShouldEqual, "supersecrettoken")
		})
	})
}
