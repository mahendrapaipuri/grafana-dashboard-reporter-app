package main

import (
	"os"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin"
	"github.com/grafana/grafana-plugin-sdk-go/backend/app"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

func main() {
	// Start listening to requests sent from Grafana. This call is blocking so
	// it won't finish until Grafana shuts down the process or the plugin choose
	// to exit by itself using os.Exit. Manage automatically manages life cycle
	// of app instances. It accepts app instance factory as first
	// argument. This factory will be automatically called on incoming request
	// from Grafana to create different instances of `App` (per plugin
	// ID).
	if err := app.Manage(plugin.Name, plugin.NewDashboardReporterApp, app.ManageOpts{}); err != nil {
		log.DefaultLogger.Error("failed to start "+plugin.Name, "err", err.Error())
		os.Exit(1)
	}
}
