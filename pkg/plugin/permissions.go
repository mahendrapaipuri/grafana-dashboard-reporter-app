package plugin

import (
	"errors"
	"net/http"
	"time"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/helpers"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/authlib/authn"
	"github.com/mahendrapaipuri/authlib/authz"
	"github.com/mahendrapaipuri/authlib/cache"
)

// HasAccess verifies if the current request context has access to certain action.
func (app *App) HasAccess(req *http.Request, action string, resources ...authz.Resource) (bool, error) {
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
	hasAccess, err := authzClient.HasAccess(req.Context(), idToken, action, resources...)
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
	var disableTypHeaderCheck bool
	if helpers.SemverCompare(app.grafanaSemVer, "v11.1.0") == -1 {
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
		// Fetch all the user permission prefixed with folders
		authz.WithSearchByPrefix("folders"),
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
