package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/worker"
)

// Javascripts vars
var (
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
	PanelPNG(ctx context.Context, dashUID string, p dashboard.Panel, t dashboard.TimeRange) (dashboard.PanelImage, error)
	PanelCSV(ctx context.Context, dashUID string, p dashboard.Panel, t dashboard.TimeRange) (dashboard.CSVData, error)
}

type Credential struct {
	HeaderName  string
	HeaderValue string
}

// GrafanaClient is the struct that will implement required interfaces
type GrafanaClient struct {
	logger         log.Logger
	conf           config.Config
	httpClient     *http.Client
	chromeInstance chrome.Instance
	workerPools    worker.Pools
	appURL         string
	credential     Credential
	queryParams    url.Values
}

// New creates a new Grafana Client. Cookies and Authorization headers, if found,
// will be forwarded in the requests
// queryParams are Grafana template variable url values of the form var-{name}={value}, e.g. var-host=dev
func New(
	logger log.Logger,
	conf config.Config,
	httpClient *http.Client,
	chromeInstance chrome.Instance,
	workerPools worker.Pools,
	appURL string,
	credential Credential,
	queryParams url.Values,
) GrafanaClient {
	return GrafanaClient{
		logger,
		conf,
		httpClient,
		chromeInstance,
		workerPools,
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
	wg := sync.WaitGroup{}
	wg.Add(2)

	// Spawn go routines to get dashboard from API and browser
	var (
		dashboardBytes     []byte
		dashboardData      []interface{}
		errBrowser, errAPI error
	)

	// Get dashboard model from API
	go func() {
		defer wg.Done()

		dashboardBytes, errAPI = g.dashboardFromAPI(ctx, dashUID)
		if errAPI != nil {
			errAPI = fmt.Errorf("error fetching dashboard from API: %w", errAPI)
		}
	}()

	// Get dashboard model from browser
	g.workerPools[worker.Browser].Do(func() {
		defer wg.Done()

		dashboardData, errBrowser = g.dashboardFromBrowser(dashUID)
		if errBrowser != nil {
			errBrowser = fmt.Errorf("error fetching dashboard from browser: %w", errBrowser)
		}
	})

	wg.Wait()

	if errBrowser != nil {
		// If the browser errors, that's fine, we can still use the API
		g.logger.Warn("error fetching dashboard from browser, falling back to API", "error", errBrowser)
	}

	if errAPI != nil {
		return dashboard.Dashboard{}, fmt.Errorf("error fetching dashboard: %w", errAPI)
	}

	// Build dashboard model from JSON and data
	grafanaDashboard, err := dashboard.New(g.logger, g.conf, dashboardBytes, dashboardData, g.queryParams)
	if err != nil {
		return dashboard.Dashboard{}, fmt.Errorf("error building dashboard model: %w", err)
	}

	return grafanaDashboard, nil
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
			"%w: URL: %s. Status: %s, message: %s",
			ErrDashboardHTTPError,
			dashURL,
			resp.Status,
			string(body),
		)
	}

	return body, nil
}

// dashboardFromBrowser fetches dashboard model from Grafana chromium browser instance
func (g GrafanaClient) dashboardFromBrowser(dashUID string) ([]interface{}, error) {
	// Get dashboard URL
	dashURL := fmt.Sprintf("%s/d/%s/_?%s", g.appURL, dashUID, g.queryParams.Encode())

	g.logger.Debug("Navigating to dashboard via browser", "url", dashURL)

	// Create a new tab
	tab := g.chromeInstance.NewTab(g.logger, g.conf)
	defer tab.Close(g.logger)

	var headers map[string]any
	if g.credential.HeaderName != "" {
		headers = map[string]any{
			g.credential.HeaderName: g.credential.HeaderValue,
		}
	}

	err := tab.NavigateAndWaitFor(dashURL, headers, "networkIdle")
	if err != nil {
		return nil, fmt.Errorf("NavigateAndWaitFor: %w", err)
	}

	tasks := make(chromedp.Tasks, 0, len(unCollapseRowsJS)+1)

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

	if len(dashboardData) == 0 {
		return nil, ErrJavaScriptReturnedNoData
	}

	return dashboardData, nil
}

func (g GrafanaClient) PanelPNG(ctx context.Context, dashUID string, p dashboard.Panel, t dashboard.TimeRange) (dashboard.PanelImage, error) {
	panelURL := g.getPanelPNGURL(p, dashUID, t)

	// Create a new request for panel
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, panelURL, nil)
	if err != nil {
		return dashboard.PanelImage{}, fmt.Errorf("error creating request for %s: %w", panelURL, err)
	}

	// Forward auth headers
	g.setCredentials(req)

	// Make request
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return dashboard.PanelImage{}, fmt.Errorf("error executing request for %s: %w", panelURL, err)
	}

	// Do multiple tries to get panel before giving up
	for retries := 1; retries < 3 && resp.StatusCode != 200; retries++ {
		resp.Body.Close()

		delay := getPanelRetrySleepTime * time.Duration(retries)
		time.Sleep(delay)
		resp, err = g.httpClient.Do(req)
		if err != nil {
			return dashboard.PanelImage{}, fmt.Errorf("error executing retry request for %s: %w", panelURL, err)
		}
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return dashboard.PanelImage{}, fmt.Errorf("error reading response body of panel PNG: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return dashboard.PanelImage{}, fmt.Errorf(
			"%w: URL: %s. Status: %s, message: %s",
			ErrDashboardHTTPError,
			panelURL,
			resp.Status,
			string(body),
		)
	}

	sb := &bytes.Buffer{}
	sb.Grow(base64.StdEncoding.EncodedLen(int(resp.ContentLength)))

	encoder := base64.NewEncoder(base64.StdEncoding, sb)

	if _, err = encoder.Write(body); err != nil {
		return dashboard.PanelImage{}, fmt.Errorf("error reading response body of panel PNG: %w", err)
	}

	return dashboard.PanelImage{
		Image:    sb.String(),
		MimeType: "image/png",
	}, nil
}

func (g GrafanaClient) getPanelPNGURL(p dashboard.Panel, dashUID string, t dashboard.TimeRange) string {
	values := url.Values{}
	values.Add("theme", g.conf.Theme)
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

func (g GrafanaClient) PanelCSV(_ context.Context, dashUID string, p dashboard.Panel, t dashboard.TimeRange) (dashboard.CSVData, error) {
	panelURL := g.getPanelCSVURL(p, dashUID, t)
	// Create a new tab
	tab := g.chromeInstance.NewTab(g.logger, g.conf)
	tab.WithTimeout(300 * time.Second)
	defer tab.Close(g.logger)

	var headers map[string]any
	if g.credential.HeaderName != "" {
		headers = map[string]any{
			g.credential.HeaderName: g.credential.HeaderValue,
		}
	}

	g.logger.Debug("Navigating to dashboard via browser", "url", panelURL)

	err := tab.NavigateAndWaitFor(panelURL, headers, "networkIdle")
	if err != nil {
		return "", fmt.Errorf("NavigateAndWaitFor: %w", err)
	}

	// this will be used to capture the request id for matching network events
	var requestID network.RequestID

	// set up a channel, so we can block later while we monitor the download
	// progress
	errCh := make(chan error, 1)

	go func() {
		// set up a listener to watch the network events and close the channel when
		// complete the request id matching is important both to filter out
		// unwanted network events and to reference the downloaded file later
		chromedp.ListenTarget(tab.Context(), func(v interface{}) {
			switch ev := v.(type) {
			case *network.EventRequestWillBeSent:
				g.logger.Debug(fmt.Sprintf("EventRequestWillBeSent: %v: %v", ev.RequestID, ev.Request.URL))
				if ev.Request.URL == "" {
					requestID = ev.RequestID
				}
			case *network.EventLoadingFinished:
				g.logger.Debug(fmt.Sprintf("EventLoadingFinished: %v", ev.RequestID))
				if ev.RequestID == requestID {
					close(errCh)
				}
			case *browser.EventDownloadProgress:
				completed := "(unknown)"
				if ev.TotalBytes != 0 {
					completed = fmt.Sprintf("%0.2f%%", ev.ReceivedBytes/ev.TotalBytes*100.0)
				}
				g.logger.Debug(fmt.Sprintf("state: %s, completed: %s", ev.State.String(), completed))
				if ev.State == browser.DownloadProgressStateCompleted {
					// Done
				}
			}
		})

		tasks := chromedp.Tasks{
			browser.
				SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
				WithDownloadPath("/root/mahendrapaipuri-dashboardreporter-app/").
				WithEventsEnabled(true),
			chromedp.WaitVisible(`div[aria-label="Panel inspector Data content"] button[type="button"][aria-disabled="false"]`, chromedp.ByQuery),
			chromedp.Click(`div[role='dialog'] button[aria-expanded=false]`, chromedp.ByQuery),
			chromedp.QueryAfter(`div[data-testid="dataOptions"] input:not(#excel-toggle):not(#formatted-data-toggle) + label`, func(ctx context.Context, execCtx runtime.ExecutionContextID, nodes ...*cdp.Node) error {
				if len(nodes) == 0 {
					return nil
				}

				return chromedp.MouseClickNode(nodes[0]).Do(ctx)
			}, chromedp.NodeVisible, chromedp.ByQuery),
			chromedp.Click(`div[aria-label="Panel inspector Data content"] button[type="button"][aria-disabled="false"]`, chromedp.ByQuery),
			chromedp.Sleep(10 * time.Second),
		}

		if err := tab.Run(tasks); err != nil {
			errCh <- fmt.Errorf("error fetching dashboard URL from browser %s: %w", panelURL, err)
		}
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case <-tab.Context().Done():
		return "", fmt.Errorf("error fetching CSV data from URL from browser %s: %w", panelURL, tab.Context().Err())
	}

	// get the downloaded bytes for the request id
	var buf []byte
	if err := tab.Run(chromedp.ActionFunc(func(ctx context.Context) error {
		var err error

		buf, err = network.GetResponseBody(requestID).Do(ctx)

		return err
	})); err != nil {
		return "", fmt.Errorf("error fetching CSV data from URL from browser %s: %w", panelURL, err)
	}

	return dashboard.CSVData(buf), nil
}

func (g GrafanaClient) getPanelCSVURL(p dashboard.Panel, dashUID string, t dashboard.TimeRange) string {
	values := url.Values{}
	values.Add("theme", g.conf.Theme)
	values.Add("viewPanel", strconv.Itoa(p.ID))
	values.Add("from", t.From)
	values.Add("to", t.To)
	values.Add("inspect", strconv.Itoa(p.ID))
	values.Add("inspectTab", "data")

	// Add templated queryParams to URL
	for k, v := range g.queryParams {
		for _, singleValue := range v {
			values.Add(k, singleValue)
		}
	}

	// Get Panel API endpoint
	return fmt.Sprintf("%s/d/%s/_?%s", g.appURL, dashUID, values.Encode())
}
