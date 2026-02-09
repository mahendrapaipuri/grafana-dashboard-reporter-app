package dashboard

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strconv"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/helpers"
)

var getPanelRetrySleepTime = time.Duration(10) * time.Second

// PanelPNG returns encoded PNG image of a given panel.
func (d *Dashboard) PanelPNG(ctx context.Context, p Panel) (PanelImage, error) {
	if d.conf.NativeRendering {
		return d.panelPNGNativeRenderer(ctx, p)
	}

	return d.panelPNGImageRenderer(ctx, p)
}

// panelPNGNativeRenderer returns panel PNG data by capturing screenshot of panel in browser.
func (d *Dashboard) panelPNGNativeRenderer(_ context.Context, p Panel) (PanelImage, error) {
	// Get panel URL
	panelURL := d.panelPNGURL(p, false)

	defer helpers.TimeTrack(time.Now(), "fetch panel PNG", d.logger, "panel_id", p.ID, "renderer", "native", "url", panelURL)

	// Create a new tab
	tab := d.chromeInstance.NewTab(d.logger, d.conf)

	tab.WithTimeout(2 * d.conf.HTTPClientOptions.Timeouts.Timeout)
	defer tab.Close(d.logger)

	headers := make(map[string]any)

	for name, values := range d.authHeader {
		for _, value := range values {
			headers[name] = value
		}
	}

	err := tab.NavigateAndWaitFor(panelURL, headers, "networkIdle", nil)
	if err != nil {
		return PanelImage{}, fmt.Errorf("NavigateAndWaitFor: %w", err)
	}

	var buf []byte

	tasks := make(chromedp.Tasks, 0)

	js := fmt.Sprintf(
		`waitForQueriesAndVisualizations(version = '%s', timeout = %d);`,
		d.appVersion, d.conf.HTTPClientOptions.Timeouts.Timeout.Milliseconds(),
	)

	tasks = append(tasks, chromedp.Tasks{
		chromedp.Evaluate(d.jsContent, nil),
		chromedp.EmulateViewport(d.panelDims(p)),
		chromedp.Evaluate(js, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.CaptureScreenshot(&buf),
	}...)

	err = tab.Run(tasks)
	if err != nil {
		return PanelImage{}, fmt.Errorf("error fetching panel PNG from browser %s: %w", panelURL, err)
	}

	sb := &bytes.Buffer{}

	encoder := base64.NewEncoder(base64.StdEncoding, sb)

	_, err = encoder.Write(buf)
	if err != nil {
		return PanelImage{}, fmt.Errorf("error reading data of panel PNG: %w", err)
	}

	return PanelImage{
		Image:    sb.String(),
		MimeType: "image/png",
	}, nil
}

// panelPNGImageRenderer returns panel PNG data by making API requests to grafana-image-renderer.
func (d *Dashboard) panelPNGImageRenderer(ctx context.Context, p Panel) (PanelImage, error) {
	// Get panel render URL
	panelURL := d.panelPNGURL(p, true)

	defer helpers.TimeTrack(time.Now(), "fetch panel PNG", d.logger, "panel_id", p.ID, "renderer", "grafana-image-renderer", "url", panelURL)

	// Create a new request for panel
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, panelURL, nil)
	if err != nil {
		return PanelImage{}, fmt.Errorf("error creating request for %s: %w", panelURL, err)
	}

	// Forward auth headers
	for name, values := range d.authHeader {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Make request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return PanelImage{}, fmt.Errorf("error executing request for %s: %w", panelURL, err)
	}
	defer resp.Body.Close()

	// Do multiple tries to get panel before giving up
	for retries := 1; retries < 3 && resp.StatusCode != http.StatusOK; retries++ {
		resp.Body.Close()

		delay := getPanelRetrySleepTime * time.Duration(retries)
		time.Sleep(delay)

		resp, err = d.httpClient.Do(req)
		if err != nil {
			return PanelImage{}, fmt.Errorf("error executing retry request for %s: %w", panelURL, err)
		}
		defer resp.Body.Close()
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PanelImage{}, fmt.Errorf("error reading response body of panel PNG: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return PanelImage{}, fmt.Errorf(
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

	_, err = encoder.Write(body)
	if err != nil {
		return PanelImage{}, fmt.Errorf("error reading response body of panel PNG: %w", err)
	}

	return PanelImage{
		Image:    sb.String(),
		MimeType: "image/png",
	}, nil
}

// panelPNGURL returns the URL to fetch panel PNG.
func (d *Dashboard) panelPNGURL(p Panel, render bool) string {
	values := maps.Clone(d.model.Dashboard.Variables)
	pID := p.ID

	// UPDATE: 20260209
	// No need of any dirty hacking as we get the supposedly correct panel IDs directly
	// from the JS. If something does not work, it is a bug in Grafana and there is nothing
	// we can do.
	//
	// From the tests, Grafana versions 11.3.x, 11.4.x, 11.5.x have quite some bugs and
	// repeated panels and rows would not work well. Later versions of >= 11.6.x seem to
	// be working very well.
	//
	// switch {
	// case helpers.SemverCompare(d.appVersion, "v11.3.0") >= 0 && helpers.SemverCompare(d.appVersion, "v11.5.0") <= 0:
	// 	// Starting from Grafana v11.3.0, repeated panels will have clone in
	// 	// their IDs.
	// 	//
	// 	// For Grafana versions between 11.3.0 and 11.5.0 (both included),
	// 	// but we need to strip it down and repeat the panel ID with
	// 	// each var. If var is $__all, there is nothing we can do, just pass
	// 	// it through and all panels will be overlapped.
	// 	//
	// 	// Panel IDs will be of format panel-10-clone-0
	// 	// Split by -clone- and get first and second parts
	// 	//
	// 	// NOTE: Workaround until https://github.com/grafana/grafana/issues/108754 gets fixed
	// 	parts := strings.Split(p.ID, "-clone-")

	// 	if p.Repeat != "" && len(parts) == 2 {
	// 		pID = parts[0]

	// 		i, err := strconv.Atoi(parts[1])
	// 		if err == nil {
	// 			varKey := "var-" + p.Repeat
	// 			if varValues := values[varKey]; len(varValues) > 0 && i < len(varValues) {
	// 				values.Del(varKey)
	// 				values.Set(varKey, varValues[i])
	// 			}
	// 		}
	// 	}
	// case helpers.SemverCompare(d.appVersion, "v12.2.0") >= 0:
	// 	// Starting from Grafana v12.2.0, the repeated panel names should have the
	// 	// value of repeated value in the panel ID.
	// 	//
	// 	// For instance, if panel is repeated on a panel variable having values
	// 	// node, grafana and panelIDs are panel-1-clone-0, panel-1-clone-1, they
	// 	// must be renamed to node$panel-1 and grafana$panel-1.
	// 	//
	// 	// Moreover if there is a repeated row, we need to prepend the row value
	// 	// as well. For instance, if repeated row value is row1, the abov example
	// 	// becomes row1$node$panel-1, row1$grafana$panel-1.
	// 	//
	// 	// IMPORTANT: The first repeated panel would not have -clone-0 suffix. So, we
	// 	// need to take that into account
	// 	//
	// 	// We can pass all the panel vars in the URL without any issues. If var is
	// 	// $__all, there is nothing we can do, we can empty panels as we do not know
	// 	// the values of the var
	// 	parts := strings.Split(p.ID, "-clone-")

	// 	if p.Repeat != "" && len(parts) >= 1 {
	// 		pID = parts[0]

	// 		// For first panel that does not -clone-0 in the name
	// 		cloneIndex := "0"
	// 		if len(parts) == 2 {
	// 			cloneIndex = parts[1]
	// 		}

	// 		i, err := strconv.Atoi(cloneIndex)
	// 		if err == nil {
	// 			panelVarKey := "var-" + p.Repeat
	// 			if panelVarValues := values[panelVarKey]; len(panelVarValues) > 0 && i < len(panelVarValues) {
	// 				values.Del(panelVarKey)
	// 				values.Set(panelVarKey, panelVarValues[i])
	// 				pID = fmt.Sprintf("%s$%s", panelVarValues[i], pID)

	// 				// If row is repeated as well, prepend row variable value to panel ID
	// 				if p.RowRepeat != "" {
	// 					rowVarKey := "var-" + p.RowRepeat
	// 					if rowVarValues := values[rowVarKey]; len(rowVarValues) > 0 && i < len(rowVarValues) {
	// 						values.Del(rowVarKey)
	// 						values.Set(rowVarKey, rowVarValues[i])
	// 						pID = fmt.Sprintf("%s$%s", rowVarValues[i], pID)
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}
	// }

	values.Add("theme", d.conf.Theme)
	values.Add("panelId", pID)

	if d.conf.TimeZone != "" && values.Get("timezone") == "" {
		values.Add("timezone", d.conf.TimeZone)
	}

	// Get panel dimensions
	w, h := d.panelDims(p)
	values.Add("width", strconv.FormatInt(w, 10))
	values.Add("height", strconv.FormatInt(h, 10))

	// If render is true call grafana-image-renderer API URL
	var renderer string
	if render {
		renderer = "render/"
	}

	// Get Panel API endpoint
	return fmt.Sprintf("%s/%sd-solo/%s/_?%s", d.appURL, renderer, d.model.Dashboard.UID, values.Encode())
}

// panelDims returns width and height of panel based on layout.
func (d *Dashboard) panelDims(p Panel) (int64, int64) {
	// According to Grafana docs, width scaling is ~80px and height
	// scaling is ~36px. However, on grid layout these scales render
	// panels that are too small to read. With some trial and error
	// we figured out that using 64px for width renders decent result
	// without too much distortion.
	//
	// From rudimentary tests, seems like Grafana cloud offering using
	// even smaller width scaling which is evident in distored aspect
	// ratios of some panels when grid layout is chosen.
	//
	// In simple layout we create panels with 1000x500 resolution always and include
	// them one in each page of report
	var width, height int64
	if d.conf.Layout == "grid" {
		width = int64(p.GridPos.W * 64)
		height = int64(p.GridPos.H * 36)
	} else {
		width = 1000
		height = 500
	}

	return width, height
}
