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
	//sanitize title. Http headers should be ASCII
	filename := strconv.QuoteToASCII(title)
	filename = strings.TrimLeft(filename, "\"")
	filename = strings.TrimRight(filename, "\"")
	header := fmt.Sprintf("inline; filename=\"%s.pdf\"", filename)
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

// /api/plugins/mahendrapaipuri-dashboardreporter-app/resources/report
// handleReport handles createing a PDF report from a given dashboard UID
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

	// Get Dashboard variables
	variables := getDashboardVariables(req)
	if len(variables) == 0 {
		ctxLogger.Debug("no variables found", "user", currentUser, "dash_uid", dashboardUID)
	}

	// Get time range
	timeRange := NewTimeRange(req.URL.Query().Get("from"), req.URL.Query().Get("to"))
	ctxLogger.Debug("time range", "range", timeRange, "user", currentUser, "dash_uid", dashboardUID)

	// Get custom settings if provided in Plugin settings
	var data map[string]interface{}
	var orientation = a.config.orientation
	var layout = a.config.layout
	var dashboardMode = a.config.dashboardMode
	var maxRenderWorkers = a.config.maxRenderWorkers
	var persistData = a.config.persistData
	if config.AppInstanceSettings.JSONData != nil {
		if err := json.Unmarshal(config.AppInstanceSettings.JSONData, &data); err == nil {
			if v, exists := data["orientation"]; exists && v.(string) != orientation {
				layout = v.(string)
				ctxLogger.Debug(
					"orientation setting",
					"orientation",
					orientation,
					"user",
					currentUser,
					"dash_uid",
					dashboardUID,
				)
			}
			if v, exists := data["layout"]; exists && v.(string) != layout {
				layout = v.(string)
				ctxLogger.Debug("layout setting", "layout", layout, "user", currentUser, "dash_uid", dashboardUID)
			}
			if v, exists := data["dashboardMode"]; exists && v.(string) != dashboardMode {
				dashboardMode = v.(string)
				ctxLogger.Debug(
					"dashboardMode setting",
					"dashboardMode",
					dashboardMode,
					"user",
					currentUser,
					"dash_uid",
					dashboardUID,
				)
			}
			if v, exists := data["maxRenderWorkers"]; exists && int(v.(float64)) != maxRenderWorkers {
				maxRenderWorkers = int(v.(float64))
				ctxLogger.Debug(
					"custom max render workers setting",
					"maxRenderWorkers",
					maxRenderWorkers,
					"user",
					currentUser,
					"dash_uid",
					dashboardUID,
				)
			}
			if v, exists := data["persistData"]; exists && v.(bool) != persistData {
				persistData = v.(bool)
				ctxLogger.Debug(
					"persistData setting",
					"persistData",
					persistData,
					"user",
					currentUser,
					"dash_uid",
					dashboardUID,
				)
			}
		}
	}

	// If layout and/or orientation and/or panels is set in query params override existing
	if queryLayouts, ok := req.URL.Query()["layout"]; ok {
		if slices.Contains([]string{"simple", "grid"}, queryLayouts[len(queryLayouts)-1]) {
			layout = queryLayouts[len(queryLayouts)-1]
		}
	}
	if queryOrientations, ok := req.URL.Query()["orientation"]; ok {
		if slices.Contains([]string{"landscape", "portrait"}, queryOrientations[len(queryOrientations)-1]) {
			orientation = queryOrientations[len(queryOrientations)-1]
		}
	}
	if queryDashboardMode, ok := req.URL.Query()["dashboardMode"]; ok {
		if slices.Contains([]string{"default", "full"}, queryDashboardMode[len(queryDashboardMode)-1]) {
			dashboardMode = queryDashboardMode[len(queryDashboardMode)-1]
		}
	}

	// Get current user cookie and we will use this cookie in API request to get
	// dashboard JSON models and panel PNGs
	cookie := req.Header.Get(backend.CookiesHeaderName)

	// Make a new Grafana client to get dashboard JSON model and Panel PNGs
	grafanaClient := a.newGrafanaClient(
		a.httpClient,
		a.grafanaAppUrl,
		cookie,
		variables,
		layout,
		dashboardMode,
	)
	// Make a new Report to put all PNGs into a LateX template and compile it into a PDF
	report, err := a.newReport(
		ctxLogger,
		grafanaClient,
		&ReportConfig{
			dashUID:          dashboardUID,
			timeRange:        timeRange,
			vfs:              a.config.vfs,
			maxRenderWorkers: maxRenderWorkers,
			layout:           layout,
			orientation:      orientation,
			persistData:      persistData,
			chromeOpts:       a.config.chromeOpts,
		},
	)
	if err != nil {
		ctxLogger.Error("error creating new Report instance", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)
		return
	}

	// Generate report
	buf, err := report.Generate()
	if err != nil {
		ctxLogger.Error("error generating report", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)
		return
	}
	if !persistData {
		defer report.Clean()
	}

	// Add PDF file name to header
	addFilenameHeader(w, report.Title())

	// Write buffered response to writer
	w.Write(buf)
	ctxLogger.Info("report generated", "user", currentUser, "dash_uid", dashboardUID)
	w.WriteHeader(http.StatusOK)
}

// registerRoutes takes a *http.ServeMux and registers some HTTP handlers.
func (a *App) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/report", a.handleReport)
}
