package plugin

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/client"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/config"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/dashboard"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/report"
)

// Add filename to Header
func addFilenameHeader(w http.ResponseWriter, title string) {
	// Sanitize title to escape non ASCII characters
	// Ref: https://stackoverflow.com/questions/62705546/unicode-characters-in-attachment-name
	// Ref: https://medium.com/@JeremyLaine/non-ascii-content-disposition-header-in-django-3a20acc05f0d
	filename := url.PathEscape(title)
	header := `inline; filename*=UTF-8''` + fmt.Sprintf("%s.pdf", filename)
	w.Header().Add("Content-Disposition", header)
}

// Get dashboard variables via query parameters
func getDashboardVariables(r *http.Request) url.Values {
	variables := url.Values{}
	for k, v := range r.URL.Query() {
		if strings.HasPrefix(k, "var-") {
			for _, singleV := range v {
				variables.Add(k, singleV)
			}
		}
	}

	return variables
}

// handleReport handles creating a PDF report from a given dashboard UID
// GET /api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report
func (app *App) handleReport(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	// Get context logger which we will use everywhere
	ctxLogger := log.DefaultLogger.FromContext(req.Context())

	// Get config from context
	pluginConfig := httpadapter.PluginConfigFromContext(req.Context())
	currentUser := pluginConfig.User.Login

	// Get Dashboard ID
	dashboardUID := req.URL.Query().Get("dashUid")
	if dashboardUID == "" {
		ctxLogger.Debug("Query parameter dashUid not found")
		http.Error(w, "Query parameter dashUid not found", http.StatusBadRequest)

		return
	}

	// Get Grafana config from context
	grafanaConfig := backend.GrafanaConfigFromContext(req.Context())
	conf, err := config.Load(
		pluginConfig.AppInstanceSettings.JSONData,
		pluginConfig.AppInstanceSettings.DecryptedSecureJSONData,
	)
	if err != nil {
		ctxLogger.Error("error loading config", "err", err)
		http.Error(w, "error loading config", http.StatusInternalServerError)

		return
	}

	if req.URL.Query().Has("layout") {
		conf.Layout = req.URL.Query().Get("layout")
		if conf.Layout != "simple" && conf.Layout != "grid" {
			ctxLogger.Debug("invalid layout parameter: " + conf.Layout)
			http.Error(w, "invalid layout parameter: "+conf.Layout, http.StatusBadRequest)

			return
		}
	}

	if req.URL.Query().Has("orientation") {
		conf.Orientation = req.URL.Query().Get("orientation")
		if conf.Orientation != "portrait" && conf.Orientation != "landscape" {
			ctxLogger.Debug("invalid orientation parameter: " + conf.Orientation)
			http.Error(w, "invalid orientation parameter: "+conf.Orientation, http.StatusBadRequest)

			return
		}
	}

	if req.URL.Query().Has("dashboardMode") {
		conf.DashboardMode = req.URL.Query().Get("dashboardMode")
		if conf.DashboardMode != "default" && conf.DashboardMode != "full" {
			ctxLogger.Warn("invalid dashboardMode parameter: " + conf.DashboardMode)
			http.Error(w, "invalid dashboardMode parameter: "+conf.DashboardMode, http.StatusBadRequest)

			return
		}
	}

	if req.URL.Query().Has("timeZone") {
		conf.TimeZone = req.URL.Query().Get("timeZone")
	}

	if req.URL.Query().Has("includePanelID") {
		conf.IncludePanelIDs = make([]int, len(req.URL.Query()["includePanelID"]))

		for i, stringID := range req.URL.Query()["includePanelID"] {
			conf.IncludePanelIDs[i], err = strconv.Atoi(stringID)
			if err != nil {
				ctxLogger.Debug("invalid includePanelID parameter: " + err.Error())
				http.Error(w, "invalid includePanelID parameter: "+err.Error(), http.StatusBadRequest)

				return
			}
		}
	}

	if req.URL.Query().Has("excludePanelID") {
		conf.ExcludePanelIDs = make([]int, len(req.URL.Query()["excludePanelID"]))

		for i, stringID := range req.URL.Query()["excludePanelID"] {
			conf.ExcludePanelIDs[i], err = strconv.Atoi(stringID)
			if err != nil {
				ctxLogger.Debug("invalid includePanelID parameter: " + err.Error())
				http.Error(w, "invalid excludePanelID parameter: "+err.Error(), http.StatusBadRequest)

				return
			}
		}
	}

	ctxLogger.Info(fmt.Sprintf("generate report using config: %s", conf.String()))

	var grafanaAppURL string
	if conf.URL != "" {
		grafanaAppURL = conf.URL
	} else {
		grafanaAppURL, err = grafanaConfig.AppURL()
		if err != nil {
			ctxLogger.Error("failed to get app URL", "err", err)
			http.Error(w, "failed to get app URL", http.StatusInternalServerError)

			return
		}
	}

	grafanaAppURL = strings.TrimSuffix(grafanaAppURL, "/")

	var credential client.Credential

	switch {
	case req.Header.Get(backend.OAuthIdentityTokenHeaderName) != "":
		credential = client.Credential{
			HeaderName:  backend.OAuthIdentityTokenHeaderName,
			HeaderValue: req.Header.Get(backend.OAuthIdentityTokenHeaderName),
		}
	case req.Header.Get(backend.OAuthIdentityTokenHeaderName) != "":
		credential = client.Credential{
			HeaderName:  backend.OAuthIdentityTokenHeaderName,
			HeaderValue: req.Header.Get(backend.OAuthIdentityTokenHeaderName),
		}
	case req.Header.Get(backend.OAuthIdentityIDTokenHeaderName) != "":
		credential = client.Credential{
			HeaderName:  backend.OAuthIdentityIDTokenHeaderName,
			HeaderValue: req.Header.Get(backend.OAuthIdentityIDTokenHeaderName),
		}
	default:
		saToken, err := grafanaConfig.PluginAppClientSecret()
		if err != nil {
			if conf.Token == "" {
				ctxLogger.Error("failed to get plugin app client secret", "err", err)
				http.Error(w, "failed to get plugin app client secret", http.StatusInternalServerError)

				return
			}

			saToken = conf.Token
		}

		credential = client.Credential{
			HeaderName:  backend.OAuthIdentityTokenHeaderName,
			HeaderValue: "Bearer " + saToken,
		}
	}

	// Get Dashboard variables
	variables := getDashboardVariables(req)
	if len(variables) == 0 {
		ctxLogger.Debug("no variables found", "user", currentUser, "dash_uid", dashboardUID)
	}

	// Get time range
	timeRange := dashboard.NewTimeRange(req.URL.Query().Get("from"), req.URL.Query().Get("to"))
	ctxLogger.Debug("time range", "range", timeRange, "user", currentUser, "dash_uid", dashboardUID)

	// Make app new Grafana client to get dashboard JSON model and Panel PNGs
	grafanaClient := client.New(
		ctxLogger,
		conf,
		app.httpClient,
		app.chromeInstance,
		app.workerPools,
		grafanaAppURL,
		credential,
		variables,
	)
	ctxLogger.Info(fmt.Sprintf("generate report using %s chrome", app.chromeInstance.Name()))

	// Make app new Report to put all PNGs into app HTML template and print it into app PDF
	pdfReport, err := report.New(
		ctxLogger,
		conf,
		app.chromeInstance,
		app.workerPools,
		grafanaClient,
		&report.Options{
			DashUID:     dashboardUID,
			Layout:      conf.Layout,
			Orientation: conf.Orientation,
			TimeRange:   timeRange,
		},
	)
	if err != nil {
		ctxLogger.Error("error creating new Report instance", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)

		return
	}

	// Add PDF file name to Header
	addFilenameHeader(w, pdfReport.Title())

	// Generate report
	if err = pdfReport.Generate(req.Context(), w); err != nil {
		ctxLogger.Error("error generating report", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)

		return
	}

	ctxLogger.Info("report generated", "user", currentUser, "dash_uid", dashboardUID)
}

// handlePing is an example HTTP GET resource that returns an OK response.
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
