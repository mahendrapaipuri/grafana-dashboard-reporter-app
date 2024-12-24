package dashboard

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"golang.org/x/mod/semver"
)

// Javascripts vars.
var (
	// JS to uncollapse rows for different Grafana versions.
	// Seems like executing JS corresponding to v10 on v11 or v11 on v10
	// does not have any side-effect, so we will always execute both of them. This
	// avoids more logic to detect Grafana version.
	unCollapseRowsJS = map[string]string{
		"v10":   `[...document.getElementsByClassName('dashboard-row--collapsed')].map((e) => e.getElementsByClassName('dashboard-row__title pointer')[0].click())`,
		"v11":   `[...document.querySelectorAll("[data-testid='dashboard-row-container']")].map((e) => [...e.querySelectorAll("[aria-expanded=false]")].map((e) => e.click()))`,
		"v11.3": `[...document.querySelectorAll("[aria-label='Expand row']")].map((e) => e.click())`,
	}

	// dashboardDataJS is a javascript to get dashboard related data.
	dashboardDataJS = `[...document.querySelectorAll('[%[1]s]')].map((e)=>({"x": e.getBoundingClientRect().x, "y": e.getBoundingClientRect().y, "width": e.getBoundingClientRect().width, "height": e.getBoundingClientRect().height, "title": e.innerText.split('\n')[0], "id": e.getAttribute("%[1]s")}))`

	// waitPanelsJS is a javascript to wait for all panels to load.
	// Seems like in Grafana v11.3.0+, panels are "lazily" loading. We need to scroll to the rows/panels for them to be visible.
	// Even after expanding rows, we need to wait for panels to load data for our `dashboardDataJS` to get the panels data.
	// It is a bit useless to wait for data to load in panels just to get the list of active panels and their positions but seems
	// like we do not have many options here.
	waitPanelsJS = `const loadPanels = async(sel = 'data-viz-panel-key', timeout = 30000) => {
	  // Define a timer to wait until next try
	  let timer = ms => new Promise(res => setTimeout(res, ms));
	  
	  // Always scroll to bottom of the page
	  window.scrollTo(0, document.body.scrollHeight);
	  
	  // Wait duration between retries
	  const waitDurationMsecs = 1000;

	  // Maximum number of checks based on timeout
	  const maxChecks = timeout / waitDurationMsecs;

	  // Initialise parameters
	  let lastPanels = [];
	  let checkCounts = 1;

	  // Panel count should be unchanged for minStableSizeIterations times
	  let countStableSizeIterations = 0;
	  const minStableSizeIterations = 3;

	  while (checkCounts++ <= maxChecks) {
	    // Get current number of panels
	 	let currentPanels = document.querySelectorAll('[' + sel + ']');

	    // If current panels and last panels are same, increment iterator
		if (lastPanels.length !== 0 && currentPanels.length === lastPanels.length) {
	      countStableSizeIterations++;
	    } else {
	      countStableSizeIterations = 0; // reset the counter
	    }

	    // If panel count is stable for minStableSizeIterations, return. We assume that
		// the dashboard has loaded with all panels
		if (countStableSizeIterations >= minStableSizeIterations) {
	      return;
	    }

	    // If not, wait and retry
		lastPanels = currentPanels;
	    await timer(waitDurationMsecs);
	  }

	  return;
	};`
)

// Tables related javascripts.
const (
	selDownloadCSVButton                             = `div[aria-label="Panel inspector Data content"] button[type="button"]`
	selInspectPanelDataTabExpandDataOptions          = `div[role='dialog'] button[aria-expanded=false]`
	selInspectPanelDataTabApplyTransformationsToggle = `div[data-testid="dataOptions"] input:not(#excel-toggle):not(#formatted-data-toggle) + label`
)

var clickDownloadCSVButton = fmt.Sprintf(`[...document.querySelectorAll('%s')].map((e)=>(e.click()))`, selDownloadCSVButton)

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
	dashboardData, err := d.panelData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard data from browser: %w", err)
	}

	// Make panels from data
	return d.createPanels(dashboardData)
}

// panelData fetches dashboard panels data from Grafana chromium browser instance.
func (d *Dashboard) panelData(_ context.Context) ([]interface{}, error) {
	// Get dashboard URL
	dashURL := fmt.Sprintf("%s/d/%s/_?%s", d.appURL, d.model.Dashboard.UID, d.model.Dashboard.Variables.Encode())

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

	// JS attribute for fetching dashboard data has changed in v11.3.0
	var dashDataJS string
	if semver.Compare(d.appVersion, "v11.3.0") == -1 {
		dashDataJS = fmt.Sprintf(dashboardDataJS, "data-panelid")
	} else {
		dashDataJS = fmt.Sprintf(dashboardDataJS, "data-viz-panel-key")

		// Set viewport. Seems like it is crucial for Grafana v11.3.0+
		tasks = append(tasks, chromedp.EmulateViewport(viewportWidth, viewportHeight))

		// Add `loadPanels()` func to tab
		tasks = append(tasks, chromedp.Evaluate(waitPanelsJS, nil))

		// Wait for all panels to lazy load
		tasks = append(tasks, chromedp.Evaluate(`loadPanels();`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}))
	}

	// If full dashboard mode is requested, add js that uncollapses rows
	if d.conf.DashboardMode == "full" {
		for _, jsExpr := range unCollapseRowsJS {
			tasks = append(tasks, chromedp.Evaluate(jsExpr, nil))
		}

		// For Grafana v11.3.0+, wait for all expanded panels to load
		if semver.Compare(d.appVersion, "v11.3.0") > -1 {
			tasks = append(tasks, chromedp.Evaluate(`loadPanels();`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
				return p.WithAwaitPromise(true)
			}))
		}
	}

	// Fetch dashboard data
	var dashboardData []interface{}

	// var buf []byte

	// JS that will fetch dashboard model
	tasks = append(tasks, chromedp.Tasks{
		chromedp.Evaluate(dashDataJS, &dashboardData),
		// chromedp.CaptureScreenshot(&buf),
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
		allErrs error
		err     error
		panels  []Panel
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
	var maxWidth float64

	// Iterate over the slice of interfaces and build each panel
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

	return panels, allErrs
}
