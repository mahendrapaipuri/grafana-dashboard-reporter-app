package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/helpers"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/report"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/authlib/authz"
)

// GrafanaUserSignInTokenHeaderName the header name used for forwarding
// the SignIn token of a Grafana User.
// Requires idForwarded feature toggle enabled.
const GrafanaUserSignInTokenHeaderName = "X-Grafana-Id" //nolint:gosec

// Required feature flags.
const (
	accessControlFeatureFlag = "accessControlOnCall" // added in Grafana 10.4.0
	idForwardingFlag         = "idForwarding"        // added in Grafana 10.3.0
)

// convertPanelIDs returns panel IDs based on Grafana version.
func (app *App) convertPanelIDs(ids []string) []string {
	// For Grafana < 11.3.0, we can use the IDs as such
	if helpers.SemverCompare(app.grafanaSemVer, "v11.3.0") == -1 {
		return ids
	}

	panelIDs := make([]string, len(ids))

	for i, id := range ids {
		if !strings.HasPrefix(id, "panel") {
			panelIDs[i] = "panel-" + id
		} else {
			panelIDs[i] = id
		}
	}

	return panelIDs
}

// updateConfig updates the default config from query parameters.
func (app *App) updateConfig(req *http.Request, conf *config.Config) {
	if req.URL.Query().Has("theme") {
		conf.Theme = req.URL.Query().Get("theme")
	}

	if req.URL.Query().Has("layout") {
		conf.Layout = req.URL.Query().Get("layout")
	}

	if req.URL.Query().Has("orientation") {
		conf.Orientation = req.URL.Query().Get("orientation")
	}

	if req.URL.Query().Has("dashboardMode") {
		conf.DashboardMode = req.URL.Query().Get("dashboardMode")
	}

	if req.URL.Query().Has("timeZone") {
		conf.TimeZone = req.URL.Query().Get("timeZone")
	}

	// Starting from Grafana v11.3.0, Grafana sets timezone query parameter.
	// We should give priority to that over the plugin's config value.
	// We will still support plugin's config parameter for backwards compatibility
	if req.URL.Query().Has("timezone") {
		timeZone := req.URL.Query().Get("timezone")
		if !slices.Contains([]string{"browser", "default"}, timeZone) {
			if timeZone == "utc" {
				timeZone = "Etc/UTC"
			}

			conf.TimeZone = timeZone
		}
	}

	if req.URL.Query().Has("timeFormat") {
		conf.TimeFormat = req.URL.Query().Get("timeFormat")
	}

	if req.URL.Query().Has("includePanelID") {
		conf.IncludePanelIDs = app.convertPanelIDs(req.URL.Query()["includePanelID"])
	}

	if req.URL.Query().Has("excludePanelID") {
		conf.ExcludePanelIDs = app.convertPanelIDs(req.URL.Query()["excludePanelID"])
	}

	if req.URL.Query().Has("includePanelDataID") {
		conf.IncludePanelDataIDs = app.convertPanelIDs(req.URL.Query()["includePanelDataID"])
	}
}

// featureTogglesEnabled checks if the necessary feature toogles are enabled on Grafana server.
func (app *App) featureTogglesEnabled(ctx context.Context) bool {
	// If Grafana <= 10.4.3, we use cookies to make request. Moreover feature toggles are
	// not available for these Grafana versions.
	if helpers.SemverCompare(app.grafanaSemVer, "v10.4.3") <= -1 {
		return false
	}

	// Get Grafana config from context
	cfg := backend.GrafanaConfigFromContext(ctx)

	// For grafana >= 10.4.4 check for feature toggles
	if cfg.FeatureToggles().IsEnabled(accessControlFeatureFlag) && cfg.FeatureToggles().IsEnabled(idForwardingFlag) {
		return true
	}

	return false
}

// grafanaAppURL returns the Grafana's App URL. User configured URL has higher
// precedence than the App URL in the request's context.
func (app *App) grafanaAppURL(grafanaConfig *backend.GrafanaCfg) (string, error) {
	var grafanaAppURL string

	var err error

	if app.conf.AppURL != "" {
		grafanaAppURL = app.conf.AppURL
	} else {
		grafanaAppURL, err = grafanaConfig.AppURL()
		if err != nil {
			return "", err
		}
	}

	return strings.TrimSuffix(grafanaAppURL, "/"), nil
}

// dashboardModel fetches dashboard JSON model from Grafana API.
func (app *App) dashboardModel(ctx context.Context, appURL, dashUID string, authHeader http.Header, values url.Values) (*dashboard.Model, error) {
	dashURL := fmt.Sprintf("%s/api/dashboards/uid/%s", appURL, dashUID)

	// Create a new GET request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dashURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for %s: %w", dashURL, err)
	}

	// Forward auth headers
	for name, values := range authHeader {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Make request
	resp, err := app.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request for %s: %w", dashURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body from %s: %w", dashURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"failed to fetch dashboard model: URL: %s. Status: %s, message: %s",
			dashURL,
			resp.Status,
			string(body),
		)
	}

	var model dashboard.Model

	// Read data into dashboard.Model
	err = json.Unmarshal(body, &model) //nolint:musttag
	if err != nil {
		return nil, fmt.Errorf("error reading response body into dashboard model: %w", err)
	}

	// Add template variables to model
	model.Dashboard.Variables = values

	return &model, nil
}

// handleReport handles creating a PDF report from a given dashboard UID
// GET /api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report.
func (app *App) handleReport(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	var err error

	// Always start with an instance of current app's config
	conf := app.conf

	// Get context logger which we will use everywhere
	ctxLogger := log.DefaultLogger.FromContext(req.Context())

	// Get config from context
	pluginConfig := backend.PluginConfigFromContext(req.Context())
	currentUser := pluginConfig.User.Login

	// Get Dashboard ID
	dashboardUID := req.URL.Query().Get("dashUid")
	if dashboardUID == "" {
		ctxLogger.Debug("Query parameter dashUid not found")
		http.Error(w, "missing dashUid query parameter", http.StatusBadRequest)

		return
	}

	// Add dash uid and user to logger
	ctxLogger = ctxLogger.With("user", currentUser, "dash_uid", dashboardUID)

	grafanaConfig := backend.GrafanaConfigFromContext(req.Context())

	// Get Grafana App URL by looking both at passed config and user defined config
	grafanaAppURL, err := app.grafanaAppURL(grafanaConfig)
	if err != nil {
		ctxLogger.Error("failed to get app URL", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)

		return
	}

	// Update plugin's config from query params
	app.updateConfig(req, &conf)

	// Validate new updated config
	if err := conf.Validate(); err != nil {
		ctxLogger.Debug("invalid config: "+conf.String(), "err", err)
		http.Error(w, "invalid query parameters found", http.StatusBadRequest)

		return
	}

	ctxLogger.Info("generate report using config: " + conf.String())

	// authHeader is header name value pair that will be used in API requests
	authHeader := http.Header{}

	switch {
	// This case is irrelevant starting from Grafana 10.4.4.
	// This commit https://github.com/grafana/grafana/commit/56a4af87d706087ea42780a79f8043df1b5bc3ea
	// made changes to not forward the cookies to app plugins.
	// So we will not be able to use cookies to make requests to Grafana to fetch
	// dashboards.
	case req.Header.Get(backend.CookiesHeaderName) != "":
		ctxLogger.Debug("using user cookie")

		authHeader.Add(backend.CookiesHeaderName, req.Header.Get(backend.CookiesHeaderName))
	case conf.Token != "":
		ctxLogger.Debug("using user configured token")

		authHeader.Add(backend.OAuthIdentityTokenHeaderName, "Bearer "+conf.Token)
	default:
		ctxLogger.Debug("using service account token")

		saToken, err := grafanaConfig.PluginAppClientSecret()
		if err != nil {
			ctxLogger.Error("failed to get plugin app client secret", "err", err)
			http.Error(w, "error generating report", http.StatusInternalServerError)

			return
		}

		if saToken == "" {
			ctxLogger.Error("failed to get plugin app client secret", "err", "empty client secret")
			http.Error(w, "error generating report", http.StatusInternalServerError)

			return
		}

		authHeader.Add(backend.OAuthIdentityTokenHeaderName, "Bearer "+saToken)
	}

	// Get dashboard JSON model from API
	model, err := app.dashboardModel(req.Context(), grafanaAppURL, dashboardUID, authHeader, req.URL.Query())
	if err != nil {
		ctxLogger.Error("failed to get dashboard JSON model", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)

		return
	}

	// If dashboard is in a folder, check if user has permissions on either the dashboard
	// or the folder.
	resources := []authz.Resource{
		{
			Kind: "dashboards",
			Attr: "uid",
			ID:   dashboardUID,
		},
	}
	if model.Meta.FolderUID != "" {
		resources = append(resources, authz.Resource{
			Kind: "folders",
			Attr: "uid",
			ID:   model.Meta.FolderUID,
		})
	}

	// If the required feature flags are enabled, check if user has access to the resource
	// using authz client.
	// Here we check if user has permissions to do an action "dashboards:read" on
	// dashboards resource of a given dashboard UID
	if app.featureTogglesEnabled(req.Context()) {
		if hasAccess, err := app.HasAccess(
			req, "dashboards:read",
			resources...,
		); err != nil || !hasAccess {
			if err != nil {
				ctxLogger.Error("failed to check permissions", "err", err)
			} else {
				ctxLogger.Error("user does not have necessary permissions to view dashboard")
			}

			http.Error(w, "permission denied", http.StatusForbidden)

			return
		}
	}

	grafanaDashboard, err := dashboard.New(
		ctxLogger,
		&conf,
		app.httpClient,
		app.chromeInstance,
		grafanaAppURL,
		app.grafanaSemVer,
		model,
		authHeader,
	)
	if err != nil {
		ctxLogger.Error("failed to create a new dashboard", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)

		return
	}

	ctxLogger.Info(fmt.Sprintf("generate report using %s chrome", app.chromeInstance.Name()))

	// Make app new Report to put all PNGs into app HTML template and print it into app PDF
	pdfReport := report.New(
		ctxLogger,
		&conf,
		app.httpClient,
		app.chromeInstance,
		app.workerPools,
		grafanaDashboard,
	)

	// Generate report
	if err = pdfReport.Generate(req.Context(), w); err != nil {
		ctxLogger.Error("error generating report", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)

		return
	}

	ctxLogger.Info("report generated")
}

// handleHealth is an example HTTP GET resource that returns an OK response.
func (app *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "text/plan")

	if _, err := w.Write([]byte("OK")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// registerRoutes takes a *http.ServeMux and registers some HTTP handlers.
func (app *App) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/report", app.handleReport)
	mux.HandleFunc("/healthz", app.handleHealth)
}
