package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/chrome"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/config"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/dashboard"
)

// Javascripts vars
var (
	errorLock = sync.RWMutex{}
	// JS to uncollapse rows for different Grafana versions.
	// Seems like executing JS corresponding to v10 on v11 or v11 on v10
	// does not have any side-effect, so we will always execute both of them. This
	// avoids more logic to detect Grafana version
	unCollapseRowsJS = map[string]string{
		"v10": `[...document.getElementsByClassName('dashboard-row--collapsed')].map((e) => e.getElementsByClassName('dashboard-row__title pointer')[0].click())`,
		"v11": `[...document.querySelectorAll("[data-testid='dashboard-row-container']")].map((e) => [...e.querySelectorAll("[aria-expanded=false]")].map((e) => e.click()))`,
	}
	dashboardDataJS = `[...document.getElementsByClassName('react-grid-item')].map((e) => ({"width": e.style.width, "height": e.style.height, "transform": e.style.transform, "id": e.getAttribute("data-panelid")}))`
)

var getPanelRetrySleepTime = time.Duration(10) * time.Second

// Grafana is a Grafana API httpClient
type Grafana interface {
	Dashboard(ctx context.Context, dashUID string) (dashboard.Dashboard, error)
	PanelPNG(ctx context.Context, dashUID string, p dashboard.Panel, t dashboard.TimeRange) (string, error)
}

type Credential struct {
	HeaderName  string
	HeaderValue string
}

// GrafanaClient is the struct that will implement required interfaces
type GrafanaClient struct {
	logger         log.Logger
	conf           *config.Config
	httpClient     *http.Client
	chromeInstance chrome.Instance
	appURL         string
	credential     Credential
	queryParams    url.Values
}

// New creates a new Grafana Client. Cookies and Authorization headers, if found,
// will be forwarded in the requests
// queryParams are Grafana template variable url values of the form var-{name}={value}, e.g. var-host=dev
func New(
	logger log.Logger,
	conf *config.Config,
	httpClient *http.Client,
	chromeInstance chrome.Instance,
	appURL string,
	credential Credential,
	queryParams url.Values,
) GrafanaClient {
	return GrafanaClient{
		logger,
		conf,
		httpClient,
		chromeInstance,
		appURL,
		credential,
		queryParams,
	}
}

// setCredentials sets  credentials in the HTTP request, if present
func (g GrafanaClient) setCredentials(r *http.Request) {
	if g.credential.HeaderName != "" {
		r.Header.Set(g.credential.HeaderName, g.credential.HeaderValue)
	}
}

// Dashboard fetches dashboard from Grafana
func (g GrafanaClient) Dashboard(ctx context.Context, dashUID string) (dashboard.Dashboard, error) {
	// Start a wait group
	wg := &sync.WaitGroup{}
	wg.Add(2)

	// Spawn go routines to get dashboard from API and browser
	var dashboardBytes []byte
	var dashboardData []interface{}
	var allErrs, err error
	// Get dashboard model from API
	go func() {
		dashboardBytes, err = g.dashboardFromAPI(ctx, dashUID)
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
		dashboardData, err = g.dashboardFromBrowser(ctx, dashUID)
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
	grafanaDashboard, err := dashboard.New(dashboardBytes, dashboardData, g.queryParams, g.conf)
	if err != nil {
		allErrs = errors.Join(err, allErrs)
	}

	return grafanaDashboard, allErrs
}

// dashboardFromAPI fetches dashboard JSON model from Grafana API and returns response body.
func (g GrafanaClient) dashboardFromAPI(ctx context.Context, dashUID string) ([]byte, error) {
	dashURL := fmt.Sprintf("%s/api/dashboards/uid/%s", g.appURL, dashUID)

	// Create a new GET request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dashURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for %s: %w", dashURL, err)
	}

	// Forward auth headers
	g.setCredentials(req)

	// Make request
	resp, err := g.httpClient.Do(req)
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
			"error obtaining dashboard from %s. Got Status %v, message: %v",
			dashURL,
			resp.Status,
			string(body),
		)
	}
	return body, nil
}

// dashboardFromBrowser fetches dashboard model from Grafana chromium browser instance
func (g GrafanaClient) dashboardFromBrowser(ctx context.Context, dashUID string) ([]interface{}, error) {
	// Get dashboard URL
	dashURL := fmt.Sprintf("%s/d/%s/_?%s", g.appURL, dashUID, g.queryParams.Encode())

	// Create a new tab
	tab := g.chromeInstance.NewTab(ctx, g.logger, g.conf)
	defer tab.Close()

	headers := map[string]any{}
	if g.credential.HeaderName != "" {
		headers[g.credential.HeaderName] = g.credential.HeaderValue
	}

	tasks := make(chromedp.Tasks, 0, 4+len(unCollapseRowsJS)+1)

	tasks = append(tasks, chrome.SetHeaders(dashURL, headers)...)

	// If full dashboard mode is requested, add js that uncollapses rows
	if g.conf.DashboardMode == "full" {
		for _, jsExpr := range unCollapseRowsJS {
			tasks = append(tasks, chromedp.Evaluate(jsExpr, nil))
		}
	}

	// Fetch dashboard data
	var dashboardData []interface{}

	// JS that will fetch dashboard model
	tasks = append(tasks, chromedp.Evaluate(dashboardDataJS, &dashboardData))
	if err := tab.Run(tasks); err != nil {
		return nil, fmt.Errorf("error fetching dashboard URL from browser %s: %w", dashURL, err)
	}

	return dashboardData, nil
}

func (g GrafanaClient) PanelPNG(ctx context.Context, dashUID string, p dashboard.Panel, t dashboard.TimeRange) (string, error) {
	panelURL := g.getPanelURL(p, dashUID, t)

	// Create a new request for panel
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, panelURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request for %s: %v", panelURL, err)
	}

	// Forward auth headers
	g.setCredentials(req)

	// Make request
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error executing request for %s: %v", panelURL, err)
	}

	// Do multiple tries to get panel before giving up
	for retries := 1; retries < 3 && resp.StatusCode != 200; retries++ {
		resp.Body.Close()

		delay := getPanelRetrySleepTime * time.Duration(retries)
		time.Sleep(delay)
		resp, err = g.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("error executing retry request for %s: %v", panelURL, err)
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("error rendering panel: %s", resp.Status)
	}

	sb := &bytes.Buffer{}
	sb.Grow(base64.StdEncoding.DecodedLen(int(resp.ContentLength)))

	decoder := base64.NewDecoder(base64.StdEncoding, resp.Body)

	if _, err = io.Copy(sb, decoder); err != nil {
		return "", fmt.Errorf("error reading response body of panel PNG: %v", err)
	}

	return sb.String(), nil
}

func (g GrafanaClient) getPanelURL(p dashboard.Panel, dashUID string, t dashboard.TimeRange) string {
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
	if g.conf.Layout == "grid" {
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

	// Get Panel API endpoint
	return fmt.Sprintf("%s/render/d-solo/%s/_?%s", g.appURL, dashUID, values.Encode())
}
