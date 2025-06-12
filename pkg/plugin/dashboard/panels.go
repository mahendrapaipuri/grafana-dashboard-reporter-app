package dashboard

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/helpers"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// Regex for parsing X and Y co-ordinates from CSS
// Scales for converting width and height to Grafana units.
//
// This is based on viewportWidth that we used in client.go which
// is 1952px. Stripping margin 32px we get 1920px / 24 = 80px
// height scale should be fine with 36px as width and aspect ratio
// should choose a height appropriately.
var (
	scales = map[string]float64{
		"width":  80,
		"height": 36,
	}
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
	// Seems like we need to use a longer height that will have all the
	// panels in the view for them to load properly. Using a regular
	// height of 1080px resulting in missing panels from JS data. So, we
	// set a "long" enough height here to cover all the panels in the dashboard
	// Need to improve this part!!
	//
	// This can be flaky though! Need to make it better in the future?!
	viewportWidth  int64 = 1952
	viewportHeight int64 = 10800
)

// panels fetches dashboard panels from Grafana chromium browser instance.
func (d *Dashboard) panels(ctx context.Context) ([]Panel, error) {
	// Fetch dashboard data from browser
	dashboardData, err := d.panelMetaData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard data from browser: %w", err)
	}

	d.logger.Debug("dashboard data fetch from browser", "data", dashboardData, "num_panels", len(dashboardData))

	// Make panels from data
	return d.createPanels(dashboardData)
}

// panelMetaData fetches dashboard panels metadata from Grafana chromium browser instance.
func (d *Dashboard) panelMetaData(_ context.Context) ([]interface{}, error) {
	// Get dashboard URL
	dashURL := fmt.Sprintf("%s/d/%s/_?%s", d.appURL, d.model.Dashboard.UID, d.model.Dashboard.Variables.Encode())

	defer helpers.TimeTrack(time.Now(), "fetch dashboard panels metadata", d.logger, "url", dashURL)

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

	err := tab.NavigateAndWaitFor(dashURL, headers, "networkIdle")
	if err != nil {
		return nil, fmt.Errorf("NavigateAndWaitFor: %w", err)
	}

	tasks := make(chromedp.Tasks, 0)

	// Fetch dashboard data
	var dashboardData []interface{}

	// var buf []byte

	js := fmt.Sprintf(
		`waitForQueriesAndVisualizations(version = '%s', mode = '%s', timeout = %d);`,
		d.appVersion, d.conf.DashboardMode, d.conf.HTTPClientOptions.Timeouts.Timeout.Milliseconds(),
	)

	// JS that will fetch dashboard model
	tasks = append(tasks, chromedp.Tasks{
		chromedp.Evaluate(d.jsContent, nil),
		chromedp.EmulateViewport(viewportWidth, viewportHeight),
		chromedp.Evaluate(js, &dashboardData, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
	}...)

	if err := tab.Run(tasks); err != nil {
		return nil, fmt.Errorf("error fetching dashboard data from browser %s: %w", dashURL, err)
	}

	// if err := os.WriteFile("fullScreenshot.png", buf, 0o644); err != nil {
	// 	d.logger.Error("failed to write screenshot", "err", err)
	// }

	if len(dashboardData) == 0 {
		return nil, ErrJavaScriptReturnedNoData
	}

	return dashboardData, nil
}

// panels creates slice of panels from the data fetched from browser's DOM model.
func (d *Dashboard) createPanels(dashData []interface{}) ([]Panel, error) {
	var (
		allErrs    error
		err        error
		panels     []Panel
		panelReprs []string
	)

	// We get HTML element's bounding box absolute coordinates which means
	// x and y start at non zero. We need to check those offsets and subtract
	// from all coordinates to ensure we start at (0, 0)
	xOffset := math.MaxFloat64
	yOffset := math.MaxFloat64

	// Seems like different versions of Grafana returns the max width differently.
	// So we check the maxWidth from returned coordinates and (w, h) tuples.
	// Max Width = Max X + Width for that element
	// We divide this maxWidth in 24 columns as done in Grafana to calculate Panel
	// coordinates
	//
	// Start off with maxWidth as viewportWidth. If not for dashboards that do not
	// occupy full width, our internal positioning method will "strech" these dashboards
	// to full width.
	maxWidth := float64(viewportWidth)

	// Iterate over the slice of interfaces and build each panel
	// Playground: https://goplay.tools/snippet/-cAljARG2Gj
	for _, panelData := range dashData {
		var p Panel

		pMap, ok := panelData.(map[string]interface{})
		if !ok {
			continue
		}

		for k, v := range pMap {
			switch v := v.(type) {
			case float64:
				switch k {
				case "width":
					p.GridPos.W = v
				case "height":
					p.GridPos.H = v
				case "x":
					p.GridPos.X = v

					if v < xOffset {
						xOffset = v
					}
				case "y":
					p.GridPos.Y = v

					if v < yOffset {
						yOffset = v
					}
				}
			case string:
				switch k {
				case "title":
					p.Title = v
				case "id":
					p.ID = v
				}
			}

			if p.GridPos.X+p.GridPos.W > maxWidth {
				maxWidth = p.GridPos.X + p.GridPos.W
			}
		}

		// If height comes to 1 or less, it is row panel and ignore it
		if math.Round(p.GridPos.H/scales["height"]) <= 1 {
			continue
		}

		// // Populate Type and Title from dashboard JSON model
		// for _, rowOrPanel := range d.model.Dashboard.RowOrPanels {
		// 	if rowOrPanel.Type == "row" {
		// 		for _, rp := range rowOrPanel.Panels {
		// 			if rp.ID == p.ID {
		// 				p.Type = rp.Type
		// 				p.Title = rp.Title

		// 				break
		// 			}
		// 		}
		// 	} else {
		// 		if p.ID == rowOrPanel.ID {
		// 			p.Type = rowOrPanel.Type
		// 			p.Title = rowOrPanel.Title

		// 			break
		// 		}
		// 	}
		// }

		// Create panel model and append to panels
		panels = append(panels, p)
		panelReprs = append(panelReprs, p.String())
	}

	// Remove xOffset and yOffset from all coordinates of panels
	// and estimate new width scale based on max width
	newScales := scales
	newScales["width"] = math.Round((maxWidth - xOffset) / 24)

	// Estimate Panel coordinates in Grafana column scale
	for ipanel := range panels {
		panels[ipanel].GridPos.X = math.Round((panels[ipanel].GridPos.X - xOffset) / scales["width"])
		panels[ipanel].GridPos.Y = math.Round((panels[ipanel].GridPos.Y - yOffset) / scales["height"])
		panels[ipanel].GridPos.W = math.Round(panels[ipanel].GridPos.W / scales["width"])
		panels[ipanel].GridPos.H = math.Round(panels[ipanel].GridPos.H / scales["height"])
	}

	// Check if we fetched any panels
	if len(panels) == 0 {
		allErrs = errors.Join(err, ErrNoPanels)

		return nil, allErrs
	}

	d.logger.Debug("fetched panels", "panels", strings.Join(panelReprs, ";"))

	return panels, allErrs
}
