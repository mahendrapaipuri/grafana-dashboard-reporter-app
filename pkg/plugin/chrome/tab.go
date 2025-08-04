package chrome

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"golang.org/x/net/context"
)

// Default URLs to block.
/*
	When dashboards use datasources that rely on Grafana Live like MQTT,
	we need to unblock /api/live/ws endpoint. However, it is observed that
	Grafana does not forward Authorization header to /api/live/ws endpoint
	when rendering individual panel and thus, websocket handshake would never
	be finished and we never get panel rendered.

	Unfortunately, chromedp's fetch method does not intercept websocket
	requests to be able to inject the headers and so we cannot render
	panels with live in native mode.

	Refs:
	- https://issues.chromium.org/issues/40923369#comment9
	- https://github.com/chromedp/chromedp/issues/805
*/
var (
	defaultBlockedURLs = []string{"*/api/frontend-metrics", "*/api/live/ws", "*/api/user/*"}
)

var WithAwaitPromise = func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithAwaitPromise(true)
}

// Tab is container for a browser tab.
type Tab struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Close releases the resources of the current browser tab.
func (t *Tab) Close(logger log.Logger) {
	if t.ctx != nil {
		var err error

		// Clear browser cookies to ensure no session is left
		if err = chromedp.Run(t.ctx, network.ClearBrowserCookies()); err != nil {
			logger.Error("got error from clear browser cookies", "error", err)
		}

		if err = chromedp.Cancel(t.ctx); err != nil {
			logger.Error("got error from cancel tab context", "error", err)
		}

		if t.cancel != nil {
			t.cancel()
		}
	}
}

// NavigateAndWaitFor navigates to the given address and waits for the given event to be fired on the page.
func (t *Tab) NavigateAndWaitFor(addr string, headers map[string]any, eventName string, blockedURLs []string) error {
	if err := t.Run(
		// block some URLs to avoid unnecessary requests
		network.SetBlockedURLs(append(defaultBlockedURLs, blockedURLs...)),
		enableLifeCycleEvents(),
	); err != nil {
		return fmt.Errorf("error enable lifecycle events: %w", err)
	}

	if headers != nil {
		if err := t.Run(setHeaders(headers)); err != nil {
			return fmt.Errorf("error set headers: %w", err)
		}
	}

	resp, err := chromedp.RunResponse(t.ctx, chromedp.Navigate(addr))
	if err != nil {
		return fmt.Errorf("failed navigate to %s: %w", addr, err)
	}

	if resp.Status != http.StatusOK {
		return fmt.Errorf("status code is %d:%s", resp.Status, resp.StatusText)
	}

	if err = t.Run(waitFor(eventName)); err != nil {
		return fmt.Errorf("error waiting for %s on page %s: %w", eventName, addr, err)
	}

	return nil
}

// WithTimeout set the timeout for the actions in the current tab.
func (t *Tab) WithTimeout(timeout time.Duration) {
	t.ctx, t.cancel = context.WithTimeout(t.ctx, timeout)
}

// Run executes the actions in the current tab.
func (t *Tab) Run(actions ...chromedp.Action) error {
	return chromedp.Run(t.ctx, actions...)
}

// RunWithTimeout executes the actions in the current tab.
func (t *Tab) RunWithTimeout(timeout time.Duration, actions ...chromedp.Action) error {
	ctx, cancel := context.WithTimeout(t.ctx, timeout)
	err := chromedp.Run(ctx, actions...)

	cancel()

	return err //nolint:wrapcheck
}

// Context returns the current tab's context.
func (t *Tab) Context() context.Context {
	return t.ctx
}

// Target returns tab's target ID.
func (t *Tab) Target() *chromedp.Target {
	return chromedp.FromContext(t.Context()).Target
}

// PrintToPDF returns chroms tasks that print the requested HTML into a PDF and returns the PDF stream handle.
func (t *Tab) PrintToPDF(options PDFOptions, writer io.Writer) error {
	err := chromedp.Run(t.ctx, chromedp.Tasks{
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to get frame tree: %w", err)
			}

			return page.SetDocumentContent(frameTree.Frame.ID, options.Body).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var (
				err        error
				pageParams *page.PrintToPDFParams
			)

			// In CI mode do not add header and footer for visual comparison
			if os.Getenv("__REPORTER_APP_CI_MODE") == "true" {
				pageParams = page.PrintToPDF().
					WithPreferCSSPageSize(true)
			} else {
				pageParams = page.PrintToPDF().
					// The unit of the size is "inch".
					// 8.28 x 11.7 is the size of an A4 paper.
					// We should able to make it configurable.
					// WithPaperWidth(8.28).
					// WithPaperHeight(11.7).
					WithDisplayHeaderFooter(true).
					WithHeaderTemplate(options.Header).
					WithFooterTemplate(options.Footer).
					WithPreferCSSPageSize(true)
			}

			pageParams = pageParams.WithTransferMode(page.PrintToPDFTransferModeReturnAsStream)

			// If landscape add it to page params
			if options.Orientation == "landscape" {
				pageParams = pageParams.WithLandscape(true)
			}

			pageParams = pageParams.WithPrintBackground(true)

			// Finally execute and get PDF buffer
			_, stream, err := pageParams.Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to print to PDF: %w", err)
			}

			reader := NewStreamReader(ctx, stream)
			defer reader.Close()

			if _, err = io.Copy(writer, reader); err != nil {
				return fmt.Errorf("failed to copy PDF stream: %w", err)
			}

			return nil
		}),
	})
	if err != nil {
		return fmt.Errorf("error rendering PDF: %w", err)
	}

	return nil
}
