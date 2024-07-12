package plugin

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

/*
	This file contains chromedp package related helper functions.
	Sources:
		- https://github.com/chromedp/chromedp/issues/1044
		- https://github.com/chromedp/chromedp/issues/431#issuecomment-592950397
		- https://github.com/chromedp/chromedp/issues/87
		- https://github.com/chromedp/examples/tree/master
*/

// newBrowserInstance allocates a new chromium browser, starts the instance
// and returns the context
func newBrowserInstance(ctx context.Context) (context.Context, func(), error) {
	// Set chrome options
	chromeOptions := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		// Seems like this is critical. When it is not turned on there are no errors
		// and plugin will exit without rendering any panels. Not sure why the error
		// handling is failing here. So, add this option as default just to avoid
		// those cases
		//
		// Ref: https://github.com/chromedp/chromedp/issues/492#issuecomment-543223861
		chromedp.Flag("ignore-certificate-errors", "1"),
	)

	// Create a new browser allocator
	/*
		The side-effect here is everytime the settings are updated from Grafana UI
		the current App instance will be disposed and a new app instance is created.
		The disposed app instance will call `Dispose()` receiver after few seconds
		which will eventually clean up browser instance.

		When there is a API request in progress, most likely that request will end up
		with context deadline error as browser instance will be cleaned up. But there
		will be a new browser straight away and subsequent request will pass.

		As it is only users with `Admin` role can update the Settings from Grafana UI
		it is not normal that these will be updated regularly. So, we can live with
		this side-effect without running into deep issues.
	*/
	allocCtx, allocCtxCancel := chromedp.NewExecAllocator(ctx, chromeOptions...)

	// start a browser (and an empty tab) so we can add more tabs to the browser
	browserCtx, browserCtxCancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(browserCtx); err != nil {
		return nil, func() {}, fmt.Errorf("couldn't create browser context: %s", err)
	}

	// To gracefully close browser and its tabs
	ctxCancelFuncs := func() {
		browserCtxCancel()
		allocCtxCancel()
	}
	return browserCtx, ctxCancelFuncs, nil
}

// printToPDF returns chroms tasks that print the requested HTML into a PDF
func printToPDF(html HTMLContent, isLandscapeOrientation bool, res *[]byte) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}

			return page.SetDocumentContent(frameTree.Frame.ID, html.body).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {

			var pageParams *page.PrintToPDFParams
			// In CI mode do not add header and footer for visual comparison
			if os.Getenv("__REPORTER_APP_CI_MODE") == "true" {
				pageParams = page.PrintToPDF().
					WithPreferCSSPageSize(true)
			} else {
				pageParams = page.PrintToPDF().
					WithDisplayHeaderFooter(true).
					WithHeaderTemplate(html.header).
					WithFooterTemplate(html.footer).
					WithPreferCSSPageSize(true)
			}

			// If landscape add it to page params
			if isLandscapeOrientation {
				pageParams = pageParams.WithLandscape(true)
			}

			// Finally execute and get PDF buffer
			buf, _, err := pageParams.Do(ctx)
			if err != nil {
				return err
			}
			*res = buf
			return nil
		}),
	}
}

// enableLifeCylceEevnts enables the chromedp life cycle events
func enableLifeCycleEvents() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		err := page.Enable().Do(ctx)
		if err != nil {
			return err
		}
		err = page.SetLifecycleEventsEnabled(true).Do(ctx)
		if err != nil {
			return err
		}
		return nil
	}
}

// navigateAndWaitFor navigates the browser to the given URL and waits for it to
// load until a given event occurs
func navigateAndWaitFor(url string, eventName string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		_, _, _, err := page.Navigate(url).Do(ctx)
		if err != nil {
			return err
		}
		return waitFor(ctx, eventName)
	}
}

// waitFor blocks until eventName is received.
// Examples of events you can wait for:
//
//	init, DOMContentLoaded, firstPaint,
//	firstContentfulPaint, firstImagePaint,
//	firstMeaningfulPaintCandidate,
//	load, networkAlmostIdle, firstMeaningfulPaint, networkIdle
//
// This is not super reliable, I've already found incidental cases where
// networkIdle was sent before load. It's probably smart to see how
// puppeteer implements this exactly.
func waitFor(ctx context.Context, eventName string) error {
	ch := make(chan struct{})
	cctx, cancel := context.WithCancel(ctx)
	chromedp.ListenTarget(cctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *page.EventLifecycleEvent:
			if e.Name == eventName {
				cancel()
				close(ch)
			}
		}
	})
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

}

// setheaders returns a task list that sets the passed headers.
func setheaders(u string, headers map[string]interface{}) chromedp.Tasks {
	return chromedp.Tasks{
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(headers)),
		enableLifeCycleEvents(),
		navigateAndWaitFor(u, "networkIdle"),
	}
}

// setcookies returns a task to navigate to a host with the passed cookies set
// on the network request.
func setcookies(u string, cookies ...string) (chromedp.Tasks, error) {
	// Throw error if cookie pairs are not passed
	if len(cookies)%2 != 0 {
		return nil, fmt.Errorf("cookie pair(s) not found")
	}

	// Get domain of current URL
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %s", err)
	}
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			// create cookie expiration
			expr := cdp.TimeSinceEpoch(time.Now().Add(time.Minute))
			// add cookies to chrome
			for i := 0; i < len(cookies); i += 2 {
				err := network.SetCookie(cookies[i], cookies[i+1]).
					WithExpires(&expr).
					WithDomain(parsedURL.Host).
					WithHTTPOnly(false).
					Do(ctx)
				if err != nil {
					return err
				}
			}
			return nil
		}),
		enableLifeCycleEvents(),
		navigateAndWaitFor(u, "networkIdle"),
	}, nil
}
