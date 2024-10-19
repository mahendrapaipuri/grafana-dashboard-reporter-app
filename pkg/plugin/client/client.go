package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/worker"
)

// Javascripts vars.
var (
	// JS to uncollapse rows for different Grafana versions.
	// Seems like executing JS corresponding to v10 on v11 or v11 on v10
	// does not have any side-effect, so we will always execute both of them. This
	// avoids more logic to detect Grafana version.
	unCollapseRowsJS = map[string]string{
		"v10": `[...document.getElementsByClassName('dashboard-row--collapsed')].map((e) => e.getElementsByClassName('dashboard-row__title pointer')[0].click())`,
		"v11": `[...document.querySelectorAll("[data-testid='dashboard-row-container']")].map((e) => [...e.querySelectorAll("[aria-expanded=false]")].map((e) => e.click()))`,
	}
	dashboardDataJS = `[...document.getElementsByClassName('react-grid-item')].map((e) => ({"width": e.style.width, "height": e.style.height, "transform": e.style.transform, "id": e.getAttribute("data-panelid")}))`
)

// Tables related javascripts.
const (
	selDownloadCSVButton                             = `div[aria-label="Panel inspector Data content"] button[type="button"][aria-disabled="false"]`
	selInspectPanelDataTabExpandDataOptions          = `div[role='dialog'] button[aria-expanded=false]`
	selInspectPanelDataTabApplyTransformationsToggle = `div[data-testid="dataOptions"] input:not(#excel-toggle):not(#formatted-data-toggle) + label`
)

// Browser vars.
var (
	// We must set a view port to browser to ensure chromedp (or chromium)
	// does not choose one randomly based on the current host.
	//
	// The idea here is to use a "regular" viewport of 1920x1080. However
	// seems like Grafana uses a 16px margin on either side and hence the
	// "effective" width of panels is only 1888px which is not a multiple of
	// 24 (which is column measure of Grafana panels). So we add that additional
	// 32px + 1920px = 1952px so that "effective" width becomes 1920px which is
	// multiple of 24. This should give us nicer panels without overlaps.
	//
	// This can be flaky though! Need to make it better in the future?!
	viewportWidth  int64 = 1952
	viewportHeight int64 = 1080
)

var getPanelRetrySleepTime = time.Duration(10) * time.Second

// Grafana is a Grafana API httpClient.
type Grafana interface {
	Dashboard(ctx context.Context, dashUID string) (dashboard.Dashboard, error)
	PanelPNG(ctx context.Context, dashUID string, p dashboard.Panel, t dashboard.TimeRange) (dashboard.PanelImage, error)
	PanelCSV(ctx context.Context, dashUID string, p dashboard.Panel, t dashboard.TimeRange) (dashboard.CSVData, error)
}

type Credential struct {
	HeaderName  string
	HeaderValue string
}

// GrafanaClient is the struct that will implement required interfaces.
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
// queryParams are Grafana template variable url values of the form var-{name}={value}, e.g. var-host=dev.
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

// setCredentials sets  credentials in the HTTP request, if present.
func (g GrafanaClient) setCredentials(r *http.Request) {
	if g.credential.HeaderName != "" {
		r.Header.Set(g.credential.HeaderName, g.credential.HeaderValue)
	}
}

// Dashboard fetches dashboard from Grafana.
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
	if reflect.DeepEqual(dashboard.Dashboard{}, grafanaDashboard) && err != nil {
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

// dashboardFromBrowser fetches dashboard model from Grafana chromium browser instance.
func (g GrafanaClient) dashboardFromBrowser(dashUID string) ([]interface{}, error) {
	// Get dashboard URL
	dashURL := fmt.Sprintf("%s/d/%s/_?%s", g.appURL, dashUID, g.queryParams.Encode())

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
	tasks = append(tasks, chromedp.EmulateViewport(viewportWidth, viewportHeight))
	tasks = append(tasks, chromedp.Evaluate(dashboardDataJS, &dashboardData))

	if err := tab.Run(tasks); err != nil {
		return nil, fmt.Errorf("error fetching dashboard URL from browser %s: %w", dashURL, err)
	}

	if len(dashboardData) == 0 {
		return nil, ErrJavaScriptReturnedNoData
	}

	return dashboardData, nil
}

// PanelPNG returns encoded PNG image of a given panel.
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
	defer resp.Body.Close()

	// Do multiple tries to get panel before giving up
	for retries := 1; retries < 3 && resp.StatusCode != http.StatusOK; retries++ {
		resp.Body.Close()

		delay := getPanelRetrySleepTime * time.Duration(retries)
		time.Sleep(delay)

		resp, err = g.httpClient.Do(req)
		if err != nil {
			return dashboard.PanelImage{}, fmt.Errorf("error executing retry request for %s: %w", panelURL, err)
		}
		defer resp.Body.Close()
	}

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

// getPanelPNGURL returns the URL to fetch panel PNG.
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

// PanelCSV returns CSV data of a given panel.
func (g GrafanaClient) PanelCSV(_ context.Context, dashUID string, p dashboard.Panel, t dashboard.TimeRange) (dashboard.CSVData, error) {
	panelURL := g.getPanelCSVURL(p, dashUID, t)
	// Create a new tab
	tab := g.chromeInstance.NewTab(g.logger, g.conf)
	// Set a timeout for the tab
	// Fail-safe for newer Grafana versions, if css has been changed.
	tab.WithTimeout(60 * time.Second)
	defer tab.Close(g.logger)

	var headers map[string]any
	if g.credential.HeaderName != "" {
		headers = map[string]any{
			g.credential.HeaderName: g.credential.HeaderValue,
		}
	}

	g.logger.Debug("fetch table data via browser", "url", panelURL)

	err := tab.NavigateAndWaitFor(panelURL, headers, "networkIdle")
	if err != nil {
		return nil, fmt.Errorf("NavigateAndWaitFor: %w", err)
	}

	// this will be used to capture the blob URL of the CSV download
	blobURLCh := make(chan string, 1)

	// If an error occurs on the way to fetching the CSV data, it will be sent to this channel
	errCh := make(chan error, 1)

	// Listen for download events. Downloading from JavaScript won't emit any network events.
	chromedp.ListenTarget(tab.Context(), func(event interface{}) {
		if eventDownloadWillBegin, ok := event.(*browser.EventDownloadWillBegin); ok {
			g.logger.Debug("got CSV download URL", "url", eventDownloadWillBegin.URL)
			// once we have the download URL, we can fetch the CSV data via JavaScript.
			blobURLCh <- eventDownloadWillBegin.URL
		}
	})

	downTasks := chromedp.Tasks{
		// Downloads needs to be allowed, otherwise the CSV request will be denied.
		// Allow download events to emit so we can get the download URL.
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath("/dev/null").
			WithEventsEnabled(true),
	}

	if err = tab.RunWithTimeout(2*time.Second, downTasks); err != nil {
		return nil, fmt.Errorf("error setting download behavior: %w", err)
	}

	if err = tab.RunWithTimeout(2*time.Second, chromedp.WaitVisible(selDownloadCSVButton, chromedp.ByQuery)); err != nil {
		return nil, fmt.Errorf("error waiting for download CSV button: %w", err)
	}

	if err = tab.RunWithTimeout(2*time.Second, chromedp.Click(selInspectPanelDataTabExpandDataOptions, chromedp.ByQuery)); err != nil {
		return nil, fmt.Errorf("error clicking on expand data options: %w", err)
	}

	if err = tab.RunWithTimeout(1*time.Second, chromedp.Click(selInspectPanelDataTabApplyTransformationsToggle, chromedp.ByQuery)); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("error clicking on apply transformations toggle: %w", err)
	}

	if err = tab.RunWithTimeout(1*time.Second, chromedp.Click(selInspectPanelDataTabApplyTransformationsToggle, chromedp.ByQuery)); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("error clicking on apply transformations toggle: %w", err)
	}

	// Run all tasks in a goroutine.
	// If an error occurs, it will be sent to the errCh channel.
	// If a element can't be found, a timeout will occur and the context will be canceled.
	go func() {
		task := chromedp.Click(selDownloadCSVButton, chromedp.ByQuery)
		if err := tab.Run(task); err != nil {
			errCh <- fmt.Errorf("error fetching dashboard URL from browser %s: %w", panelURL, err)
		}
	}()

	var blobURL string

	select {
	case blobURL = <-blobURLCh:
		if blobURL == "" {
			return nil, fmt.Errorf("error fetching CSV data from URL from browser %s: %w", panelURL, ErrEmptyBlobURL)
		}
	case err := <-errCh:
		return nil, fmt.Errorf("error fetching CSV data from URL from browser %s: %w", panelURL, err)
	case <-tab.Context().Done():
		return nil, fmt.Errorf("error fetching CSV data from URL from browser %s: %w", panelURL, tab.Context().Err())
	}

	close(blobURLCh)
	close(errCh)

	var buf []byte

	task := chromedp.Evaluate(
		// fetch the CSV data from the blob URL, using Javascript.
		fmt.Sprintf("fetch('%s').then(r => r.blob()).then(b => new Response(b).text()).then(t => t)", blobURL),
		&buf,
		chrome.WithAwaitPromise,
	)

	if err := tab.RunWithTimeout(45*time.Second, task); err != nil {
		return nil, fmt.Errorf("error fetching CSV data from URL from browser %s: %w", panelURL, err)
	}

	if len(buf) == 0 {
		return nil, fmt.Errorf("error fetching CSV data from URL from browser %s: %w", panelURL, ErrEmptyCSVData)
	}

	csvStringData, err := strconv.Unquote(string(buf))
	if err != nil {
		return nil, fmt.Errorf("error unquoting CSV data: %w", err)
	}

	reader := csv.NewReader(strings.NewReader(csvStringData))

	csvData, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV data: %w", err)
	}

	return csvData, nil
}

// getPanelCSVURL returns URL to fetch panel's CSV data.
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
