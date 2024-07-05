package plugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
)

// Add filename to header
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
func (a *App) handleReport(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get context logger which we will use everywhere
	ctxLogger := log.DefaultLogger.FromContext(req.Context())

	// Get config from context
	config := httpadapter.PluginConfigFromContext(req.Context())
	currentUser := config.User.Login

	// Get Dashboard ID
	dashboardUID := req.URL.Query().Get("dashUid")
	if dashboardUID == "" {
		http.Error(w, "Query parameter dashUid not found", http.StatusBadRequest)
		return
	}

	// Get Grafana config from context
	grafanaConfig := backend.GrafanaConfigFromContext(req.Context())
	if saToken, err := grafanaConfig.PluginAppClientSecret(); err != nil {
		ctxLogger.Warn("failed to get plugin app secret", "err", err)
	} else {
		// If we are on Grafana >= 10.3.0 and externalServiceAccounts are enabled
		// always prefer this token over the one that is configured in plugin config
		if saToken != "" {
			a.secrets.token = saToken
		}
	}

	// If cookie is found in request headers, add it to secrets as well
	if req.Header.Get(backend.CookiesHeaderName) != "" {
		ctxLogger.Debug("cookie found in the request", "user", currentUser, "dash_uid", dashboardUID)
		a.secrets.cookieHeader = req.Header.Get(backend.CookiesHeaderName)
		var cookies []string
		for _, cookie := range req.Cookies() {
			cookies = append(cookies, cookie.Name)
			cookies = append(cookies, cookie.Value)
		}
		a.secrets.cookies = cookies
	}

	// Get Dashboard variables
	variables := getDashboardVariables(req)
	if len(variables) == 0 {
		ctxLogger.Debug("no variables found", "user", currentUser, "dash_uid", dashboardUID)
	}

	// Get time range
	timeRange := NewTimeRange(req.URL.Query().Get("from"), req.URL.Query().Get("to"))
	ctxLogger.Debug("time range", "range", timeRange, "user", currentUser, "dash_uid", dashboardUID)

	// Always start with new instance of app config for each request
	a.config = currentDashboardReporterAppConfig()

	// Get custom settings if provided in Plugin settings
	// Seems like when json.RawMessage is nil, it actually returns []byte("null"). So
	// we need to check for both
	// Ref: https://go.dev/src/encoding/json/stream.go?s=6218:6240#L262
	if config.AppInstanceSettings.JSONData != nil && string(config.AppInstanceSettings.JSONData) != "null" {
		if err := json.Unmarshal(config.AppInstanceSettings.JSONData, &a.config); err != nil {
			ctxLogger.Error(
				"failed to update config", "user", currentUser, "dash_uid", dashboardUID, "err", err,
			)
		}
	}

	// Trim trailing slash in app URL (Just in case)
	a.config.AppURL = strings.TrimRight(a.config.AppURL, "/")

	// Override config if any of them are set in query parameters
	if queryLayouts, ok := req.URL.Query()["layout"]; ok {
		if slices.Contains([]string{"simple", "grid"}, queryLayouts[len(queryLayouts)-1]) {
			a.config.Layout = queryLayouts[len(queryLayouts)-1]
		}
	}
	if queryOrientations, ok := req.URL.Query()["orientation"]; ok {
		if slices.Contains([]string{"landscape", "portrait"}, queryOrientations[len(queryOrientations)-1]) {
			a.config.Orientation = queryOrientations[len(queryOrientations)-1]
		}
	}
	if queryDashboardMode, ok := req.URL.Query()["dashboardMode"]; ok {
		if slices.Contains([]string{"default", "full"}, queryDashboardMode[len(queryDashboardMode)-1]) {
			a.config.DashboardMode = queryDashboardMode[len(queryDashboardMode)-1]
		}
	}
	if timeZone, ok := req.URL.Query()["timeZone"]; ok {
		a.config.TimeZone = timeZone[len(timeZone)-1]
	}

	// Two special query parameters: includePanelID and excludePanelID
	// The query parameters are self explanatory and based on the values set to them
	// panels will be included/excluded in the final report
	a.config.IncludePanelIDs = nil
	a.config.ExcludePanelIDs = nil
	if includePanelIDs, ok := req.URL.Query()["includePanelID"]; ok {
		for _, id := range includePanelIDs {
			if idInt, err := strconv.Atoi(id); err == nil {
				a.config.IncludePanelIDs = append(a.config.IncludePanelIDs, idInt)
			}
		}
	}
	if excludePanelIDs, ok := req.URL.Query()["excludePanelID"]; ok {
		for _, id := range excludePanelIDs {
			if idInt, err := strconv.Atoi(id); err == nil {
				a.config.ExcludePanelIDs = append(a.config.ExcludePanelIDs, idInt)
			}
		}
	}
	ctxLogger.Info(
		"updated config", "config", a.config.String(), "user", currentUser, "dash_uid", dashboardUID,
	)

	// Make a new Grafana client to get dashboard JSON model and Panel PNGs
	grafanaClient := a.newGrafanaClient(
		ctxLogger,
		a.httpClient,
		a.secrets,
		a.config,
		variables,
	)

	// Make a new Report to put all PNGs into a HTML template and print it into a PDF
	report, err := a.newReport(
		ctxLogger,
		grafanaClient,
		&ReportOptions{
			dashUID:   dashboardUID,
			timeRange: timeRange,
			config:    a.config,
		},
	)
	if err != nil {
		ctxLogger.Error("error creating new Report instance", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)
		return
	}

	// Generate report
	buf, err := report.Generate(req.Context())
	if err != nil {
		ctxLogger.Error("error generating report", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)
		return
	}

	// Add PDF file name to header
	addFilenameHeader(w, report.Title(req.Context()))

	// Write buffered response to writer
	w.Write(buf)
	ctxLogger.Info("report generated", "user", currentUser, "dash_uid", dashboardUID)
	w.WriteHeader(http.StatusOK)
}

// registerRoutes takes a *http.ServeMux and registers some HTTP handlers.
func (a *App) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/report", a.handleReport)
}
