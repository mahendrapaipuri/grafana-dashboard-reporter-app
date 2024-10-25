package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/authlib/authn"
	"github.com/mahendrapaipuri/authlib/authz"
	"github.com/mahendrapaipuri/authlib/cache"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/client"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/report"
	"golang.org/x/mod/semver"
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

// Add filename to Header.
func addFilenameHeader(w http.ResponseWriter, title string) {
	// Sanitize title to escape non ASCII characters
	// Ref: https://stackoverflow.com/questions/62705546/unicode-characters-in-attachment-name
	// Ref: https://medium.com/@JeremyLaine/non-ascii-content-disposition-header-in-django-3a20acc05f0d
	filename := url.PathEscape(title)
	header := `inline; filename*=UTF-8''` + filename + ".pdf"
	w.Header().Add("Content-Disposition", header)
}

// Get dashboard variables via query parameters.
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

// featureTogglesEnabled checks if the necessary feature toogles are enabled on Grafana server.
func (app *App) featureTogglesEnabled(ctx context.Context) bool {
	// If Grafana <= 10.4.3, we use cookies to make request. Moreover feature toggles are
	// not available for these Grafana versions.
	if semver.Compare(app.grafanaSemVer, "v10.4.3") <= -1 {
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

// HasAccess verifies if the current request context has access to certain action.
func (app *App) HasAccess(req *http.Request, action string, resource authz.Resource) (bool, error) {
	// Retrieve the id token
	idToken := req.Header.Get(GrafanaUserSignInTokenHeaderName)
	if idToken == "" {
		return false, errors.New("id token not found")
	}

	authzClient, err := app.GetAuthZClient(req)
	if err != nil {
		return false, err
	}

	// Check user access
	hasAccess, err := authzClient.HasAccess(req.Context(), idToken, action, resource)
	if err != nil || !hasAccess {
		return false, err
	}

	return true, nil
}

// GetAuthZClient returns an authz enforcement client configured thanks to the plugin context.
func (app *App) GetAuthZClient(req *http.Request) (authz.EnforcementClient, error) {
	ctx := req.Context()
	ctxLogger := log.DefaultLogger.FromContext(ctx)

	// Prevent two concurrent calls from updating the client
	app.mx.Lock()
	defer app.mx.Unlock()

	grafanaConfig := backend.GrafanaConfigFromContext(req.Context())

	grafanaAppURL, err := app.grafanaAppURL(grafanaConfig)
	if err != nil {
		ctxLogger.Error("failed to get app URL", "err", err)

		return nil, err
	}

	// Bail we cannot get token provisioned by externalServiceAccount and no token
	// has been manually configured. In this case we cannot check permissions and moreover
	// we cannot make API requests to Grafana
	saToken, err := grafanaConfig.PluginAppClientSecret()
	if err != nil && app.conf.Token == "" {
		ctxLogger.Error("failed to fetch service account and configured token", "error", err)

		return nil, err
	} else {
		if saToken == "" {
			saToken = app.conf.Token
		}
	}

	if saToken == app.saToken {
		ctxLogger.Debug("token unchanged returning existing authz client")

		return app.authzClient, nil
	}

	// Header "typ" has been added only in Grafana 11.1.0 (https://github.com/grafana/grafana/pull/87430)
	// So this check will fail for Grafana < 11.1.0
	// Set VerifierConfig{DisableTypHeaderCheck: true} for those cases
	disableTypHeaderCheck := false
	if semver.Compare(app.grafanaSemVer, "v11.1.0") == -1 {
		disableTypHeaderCheck = true
	}

	// Initialize the authorization client
	client, err := authz.NewEnforcementClient(authz.Config{
		APIURL: grafanaAppURL,
		Token:  saToken,
		// Grafana is signing the JWTs on local setups
		JWKsURL: grafanaAppURL + "/api/signing-keys/keys",
	},
		// Use the configured HTTP client
		authz.WithHTTPClient(app.httpClient),
		// Configure verifier
		authz.WithVerifier(authn.NewVerifier[authz.CustomClaims](authn.VerifierConfig{
			DisableTypHeaderCheck: disableTypHeaderCheck,
		},
			authn.TokenTypeID,
			authn.NewKeyRetriever(authn.KeyRetrieverConfig{
				SigningKeysURL: grafanaAppURL + "/api/signing-keys/keys",
			},
				authn.WithHTTPClientKeyRetrieverOpt(app.httpClient)),
		)),
		// Fetch all the user permission prefixed with dashboards
		authz.WithSearchByPrefix("dashboards"),
		// Use a cache with a lower expiry time
		authz.WithCache(cache.NewLocalCache(cache.Config{
			Expiry:          10 * time.Second,
			CleanupInterval: 5 * time.Second,
		})),
	)
	if err != nil {
		ctxLogger.Error("failed to initialize authz client", "err", err)

		return nil, err
	}

	app.authzClient = client
	app.saToken = saToken

	return client, nil
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
		http.Error(w, "Query parameter dashUid not found", http.StatusBadRequest)

		return
	}

	grafanaConfig := backend.GrafanaConfigFromContext(req.Context())

	// Get Grafana App URL by looking both at passed config and user defined config
	grafanaAppURL, err := app.grafanaAppURL(grafanaConfig)
	if err != nil {
		ctxLogger.Error("failed to get app URL", "err", err)
		http.Error(w, "failed to get app URL", http.StatusInternalServerError)

		return
	}

	// If the required feature flags are enabled, check if user has access to the resource
	// using authz client.
	// Here we check if user has permissions to do an action "dashboards:read" on
	// dashboards resource of a given dashboard UID
	if app.featureTogglesEnabled(req.Context()) {
		if hasAccess, err := app.HasAccess(
			req, "dashboards:read",
			authz.Resource{
				Kind: "dashboards",
				Attr: "uid",
				ID:   dashboardUID,
			},
		); err != nil || !hasAccess {
			if err != nil {
				ctxLogger.Error("failed to check permissions", "err", err)
			}

			http.Error(w, "permission denied", http.StatusForbidden)

			return
		}
	}

	if req.URL.Query().Has("theme") {
		conf.Theme = req.URL.Query().Get("theme")
		if conf.Theme != "light" && conf.Theme != "dark" {
			ctxLogger.Debug("invalid theme parameter: " + conf.Theme)
			http.Error(w, "invalid theme parameter: "+conf.Theme, http.StatusBadRequest)

			return
		}
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

	if req.URL.Query().Has("includePanelDataID") {
		conf.IncludePanelDataIDs = make([]int, len(req.URL.Query()["includePanelDataID"]))

		for i, stringID := range req.URL.Query()["includePanelDataID"] {
			conf.IncludePanelDataIDs[i], err = strconv.Atoi(stringID)
			if err != nil {
				ctxLogger.Debug("invalid includePanelDataID parameter: " + err.Error())
				http.Error(w, "invalid includePanelDataID parameter: "+err.Error(), http.StatusBadRequest)

				return
			}
		}
	}

	ctxLogger.Info("generate report using config: " + conf.String())

	// credential is header name value pair that will be used in API requests
	var credential client.Credential

	switch {
	// This case is irrelevant starting from Grafana 10.4.4.
	// This commit https://github.com/grafana/grafana/commit/56a4af87d706087ea42780a79f8043df1b5bc3ea
	// made changes to not forward the cookies to app plugins.
	// So we will not be able to use cookies to make requests to Grafana to fetch
	// dashboards.
	case req.Header.Get(backend.CookiesHeaderName) != "":
		ctxLogger.Debug("using user cookie")

		credential = client.Credential{
			HeaderName:  backend.CookiesHeaderName,
			HeaderValue: req.Header.Get(backend.CookiesHeaderName),
		}
	case conf.Token != "":
		ctxLogger.Debug("using user configured token")

		credential = client.Credential{
			HeaderName:  backend.OAuthIdentityTokenHeaderName,
			HeaderValue: "Bearer " + conf.Token,
		}
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

	// Generate report
	if err = pdfReport.Generate(req.Context(), w); err != nil {
		ctxLogger.Error("error generating report", "err", err)
		http.Error(w, "error generating report", http.StatusInternalServerError)

		return
	}

	// Add PDF file name to Header
	addFilenameHeader(w, pdfReport.Title())

	ctxLogger.Info("report generated", "user", currentUser, "dash_uid", dashboardUID)
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
