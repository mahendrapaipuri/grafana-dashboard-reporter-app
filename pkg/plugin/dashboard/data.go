package dashboard

import (
	"context"
	"encoding/csv"
	"fmt"
	"maps"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/helpers"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// PanelCSV returns CSV data of a given panel.
func (d *Dashboard) PanelCSV(_ context.Context, p Panel) (CSVData, error) {
	// Get panel CSV data URL
	panelURL := d.panelCSVURL(p)

	defer helpers.TimeTrack(time.Now(), "fetch panel CSV data", d.logger, "fetcher", "native", "panel_id", p.ID, "url", panelURL.String())

	// Create a new tab
	tab := d.chromeInstance.NewTab(d.logger, d.conf)
	// Set a timeout for the tab
	// Fail-safe for newer Grafana versions, if css has been changed.
	tab.WithTimeout(2 * d.conf.HTTPClientOptions.Timeouts.Timeout)
	defer tab.Close(d.logger)

	headers := make(map[string]any)

	for name, values := range d.authHeader {
		for _, value := range values {
			headers[name] = value
		}
	}

	err := tab.NavigateAndWaitFor(panelURL.String(), headers, "networkIdle")
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
			d.logger.Debug("got CSV download URL", "panel_id", p.ID, "url", eventDownloadWillBegin.URL)
			// once we have the download URL, we can fetch the CSV data via JavaScript.
			blobURLCh <- eventDownloadWillBegin.URL
		}
	})

	js := fmt.Sprintf(
		`waitForCSVData(version = '%s', timeout = %d);`,
		d.appVersion, d.conf.HTTPClientOptions.Timeouts.Timeout.Milliseconds(),
	)

	downTasks := chromedp.Tasks{
		// Downloads needs to be allowed, otherwise the CSV request will be denied.
		// Allow download events to emit so we can get the download URL.
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath("/dev/null").
			WithEventsEnabled(true),
		chromedp.Evaluate(d.jsContent, nil),
		chromedp.Evaluate(js, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
	}

	// Run all tasks in a goroutine.
	// If an error occurs, it will be sent to the errCh channel.
	// If a element can't be found, a timeout will occur and the context will be canceled.
	go func() {
		if err := tab.Run(downTasks); err != nil {
			errCh <- fmt.Errorf("error fetching CSV URL from browser %s: %w", panelURL, err)
		}
	}()

	var blobURL string

	select {
	case blobURL = <-blobURLCh:
		if blobURL == "" {
			return nil, fmt.Errorf("error fetching CSV data from URL from browser %s: %w", panelURL, ErrEmptyBlobURL)
		}
	case err := <-errCh:
		return nil, fmt.Errorf("error fetching CSV data using URL from browser %s: %w", panelURL, err)
	case <-tab.Context().Done():
		return nil, fmt.Errorf("error fetching CSV data using URL from browser %s: %w", panelURL, tab.Context().Err())
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

	if err := tab.RunWithTimeout(d.conf.HTTPClientOptions.Timeouts.Timeout, task); err != nil {
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

// panelCSVURL returns URL to fetch panel's CSV data.
func (d *Dashboard) panelCSVURL(p Panel) *url.URL {
	values := maps.Clone(d.model.Dashboard.Variables)
	values.Add("theme", d.conf.Theme)
	values.Add("viewPanel", p.ID)
	values.Add("inspect", p.ID)
	values.Add("inspectTab", "data")

	// Make a copy of appURL
	panelURL := *d.appURL
	panelURL.Path = fmt.Sprintf("/d/%s/_", d.model.Dashboard.UID)
	panelURL.RawQuery = values.Encode()

	// Get Panel API endpoint
	return &panelURL
}
