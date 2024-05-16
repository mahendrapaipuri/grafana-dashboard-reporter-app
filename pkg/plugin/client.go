package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Javascripts vars
var (
	errorLock        = sync.RWMutex{}
	unCollapseRowsJS = `[...document.getElementsByClassName('dashboard-row--collapsed')].map((e) => e.getElementsByClassName('dashboard-row__title pointer')[0].click())`
	dashboardDataJS  = `[...document.getElementsByClassName('react-grid-item')].map((e) => ({"width": e.style.width, "height": e.style.height, "transform": e.style.transform, "id": e.getAttribute("data-panelid")}))`
)

// Client is a Grafana API client
type GrafanaClient interface {
	Dashboard(dashUID string) (Dashboard, error)
	PanelPNG(p Panel, dashUID string, t TimeRange) (io.ReadCloser, error)
}

// grafanaClient is the struct that will implement required interfaces
type grafanaClient struct {
	client           *http.Client
	dashAPIEndpoint  func(dashUID string) string
	panelAPIEndpoint func(dashUID string, vals url.Values) string
	dashBrowserURL   func(dashUID string) string
	secrets          *Secrets
	config           *Config
	queryParams      url.Values
}

var getPanelRetrySleepTime = time.Duration(10) * time.Second

// NewClient creates a new Grafana Client. Cookies and Authorization headers, if found,
// will be forwarded in the requests
// queryParams are Grafana template variable url values of the form var-{name}={value}, e.g. var-host=dev
func NewGrafanaClient(
	client *http.Client,
	secrets *Secrets,
	config *Config,
	queryParams url.Values,
) GrafanaClient {
	// Get dashboard API endpoint
	dashAPIEndpoint := func(dashUID string) string {
		dashURL := fmt.Sprintf("%s/api/dashboards/uid/%s", config.AppURL, dashUID)
		return dashURL
	}

	// Get dashboard URL
	dashBrowserURL := func(dashUID string) string {
		dashURL := fmt.Sprintf("%s/d/%s/_?%s", config.AppURL, dashUID, queryParams.Encode())
		return dashURL
	}

	// Get Panel API endpoint
	panelAPIEndpoint := func(dashUID string, vals url.Values) string {
		return fmt.Sprintf("%s/render/d-solo/%s/_?%s", config.AppURL, dashUID, vals.Encode())
	}
	return grafanaClient{
		client,
		dashAPIEndpoint,
		panelAPIEndpoint,
		dashBrowserURL,
		secrets,
		config,
		queryParams,
	}
}

// Forward auth header in the client request
// We fetch secrets from different sources
//   - From plugin config where users can configure a service account token either by
//     provisioning or set it iun UI
//   - If Grafana >= 10.3.0 is used and externalServiceAccounts is enabled, a token
//     will be generated for the plugin to use in API requests to Grafana. This token,
//     if found, will always have higher precendence to the one configured in the plugin
//   - Finally, if the request is coming from the browser, cookie will be retrieved.
//
// We should always prefer cookie to auth tokens as cookie will have correct permissions
// and scopes based on the user who is making the request. On the other hand, service
// account tokens, either configured from plugin or the one that is provisioned automatically
// for the plugin will always have broader scopes and permissions.
func (g grafanaClient) forwardAuthHeader(r *http.Request) *http.Request {
	// If incoming request has cookies formward them
	// If cookie is not found, try Authorization header that is either configured
	// in config or fetched from externalServiceAccount
	if g.secrets.cookieHeader != "" {
		r.Header.Set(backend.CookiesHeaderName, g.secrets.cookieHeader)
	} else if g.secrets.token != "" {
		r.Header.Set(backend.OAuthIdentityTokenHeaderName, fmt.Sprintf("Bearer %s", g.secrets.token))
	}
	return r
}

// Dashboard fetches dashboard from Grafana
func (g grafanaClient) Dashboard(dashUID string) (Dashboard, error) {
	// Start a wait group
	wg := &sync.WaitGroup{}
	wg.Add(2)

	// Spawn go routines to get dashboard from API and browser
	var dashboardBytes []byte
	var dashboardData []interface{}
	var allErrs, err error
	// Get dashboard model from API
	go func() {
		dashboardBytes, err = g.dashboardFromAPI(dashUID)
		if err != nil {
			errorLock.Lock()
			allErrs = errors.Join(err, allErrs)
			errorLock.Unlock()
		}

		// Mark routine as done
		wg.Done()
	}()

	// Get dashboard model from browser
	go func() {
		dashboardData, err = g.dashboardFromBrowser(dashUID)
		if err != nil {
			errorLock.Lock()
			allErrs = errors.Join(err, allErrs)
			errorLock.Unlock()
		}

		// Mark routine as done
		wg.Done()
	}()

	// Wait for go routines
	wg.Wait()

	// Build dashboard model from JSON and data
	dashboard, err := NewDashboard(dashboardBytes, dashboardData, g.queryParams, g.config)
	if err != nil {
		allErrs = errors.Join(err, allErrs)
	}
	return dashboard, allErrs
}

// dashboardFromAPI fetches dashboard JSON model from Grafana API and returns response body
func (g grafanaClient) dashboardFromAPI(dashUID string) ([]byte, error) {
	dashURL := g.dashAPIEndpoint(dashUID)

	// Create a new GET request
	req, err := http.NewRequest(http.MethodGet, dashURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for %s: %v", dashURL, err)
	}

	// Forward auth headers
	req = g.forwardAuthHeader(req)

	// Make request
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request for %s: %v", dashURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body from %s: %v", dashURL, err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(
			"error obtaining dashboard from %s. Got Status %v, message: %v ",
			dashURL,
			resp.Status,
			string(body),
		)
	}
	return body, nil
}

// dashboardFromBrowser fetches dashboard model from Grafana chromium browser instance
//
// NOTE: This is experimental. This should give us the most accurate dashboard model
// by including the repeated panels and/or rows
func (g grafanaClient) dashboardFromBrowser(dashUID string) ([]interface{}, error) {
	dashURL := g.dashBrowserURL(dashUID)

	// Create new context
	allocCtx, allocCtxCancel := chromedp.NewExecAllocator(context.Background(), g.config.ChromeOptions...)
	defer allocCtxCancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Always prefer cookie over token
	var tasks chromedp.Tasks
	if g.secrets.cookieValue != "" {
		tasks = setcookies(dashURL, g.config.CookieName, g.secrets.cookieValue)
	} else if g.secrets.token != "" {
		headers := map[string]interface{}{backend.OAuthIdentityTokenHeaderName: fmt.Sprintf("Bearer %s", g.secrets.token)}
		tasks = setheaders(dashURL, headers)
	}

	// Fetch dashboard data
	var unCollapseOut, dashboardData []interface{}
	if g.config.DashboardMode == "full" {
		if err := chromedp.Run(ctx,
			tasks,
			chromedp.Evaluate(unCollapseRowsJS, &unCollapseOut),
			chromedp.Evaluate(dashboardDataJS, &dashboardData),
		); err != nil {
			return nil, fmt.Errorf("error fetching dashboard URL from browser %s: %s", dashURL, err)
		}
	} else {
		if err := chromedp.Run(ctx,
			tasks,
			chromedp.Evaluate(dashboardDataJS, &dashboardData),
		); err != nil {
			return nil, fmt.Errorf("error fetching dashboard URL from browser %s: %s", dashURL, err)
		}
	}
	return dashboardData, nil
}

func (g grafanaClient) PanelPNG(p Panel, dashUID string, t TimeRange) (io.ReadCloser, error) {
	panelURL := g.getPanelURL(p, dashUID, t)

	// Create a new request for panel
	req, err := http.NewRequest(http.MethodGet, panelURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for %s: %v", panelURL, err)
	}

	// Forward auth headers
	req = g.forwardAuthHeader(req)

	// Make request
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request for %s: %v", panelURL, err)
	}

	// Do multiple tries to get panel before giving up
	for retries := 1; retries < 3 && resp.StatusCode != 200; retries++ {
		delay := getPanelRetrySleepTime * time.Duration(retries)
		time.Sleep(delay)
		resp, err = g.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error executing retry request for %s: %v", panelURL, err)
		}
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error rendering panel: %s", resp.Status)
	}
	return resp.Body, nil
}

func (g grafanaClient) getPanelURL(p Panel, dashUID string, t TimeRange) string {
	values := url.Values{}
	values.Add("theme", "light")
	values.Add("panelId", strconv.Itoa(p.ID))
	values.Add("from", t.From)
	values.Add("to", t.To)

	// If using a grid layout we use 100px for width and 36px for height scaling.
	// Grafana panels are fitted into 24 units width and height units are said to
	// 30px in docs but 36px seems to be better.
	//
	// In simple layout we create panels with 1000x500 resolution always and include
	// them one in each page of report
	if g.config.Layout == "grid" {
		width := int(p.GridPos.W * 100)
		height := int(p.GridPos.H * 36)
		values.Add("width", strconv.Itoa(width))
		values.Add("height", strconv.Itoa(height))
	} else {
		values.Add("width", "1000")
		values.Add("height", "500")
	}

	// Add templated queryParams to URL
	for k, v := range g.queryParams {
		for _, singleValue := range v {
			values.Add(k, singleValue)
		}
	}
	return g.panelAPIEndpoint(dashUID, values)
}
