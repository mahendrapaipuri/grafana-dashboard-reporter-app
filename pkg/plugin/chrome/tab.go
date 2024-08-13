package chrome

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"golang.org/x/net/context"
)

// Tab is container for a browser tab
type Tab struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Close releases the resources of the current browser tab
func (t *Tab) Close(logger log.Logger) {
	if t.cancel != nil {
		t.cancel()
	}

	if t.ctx != nil {
		var err error

		// Clear browser cookies to ensure no session is left
		if err = chromedp.Run(t.ctx, network.ClearBrowserCookies()); err != nil {
			logger.Error("got error from clear browser cookies", "error", err)
		}

		if err = chromedp.Cancel(t.ctx); err != nil {
			logger.Error("got error from cancel tab context", "error", err)
		}
	}
}

// NavigateAndWaitFor navigates to the given address and waits for the given event to be fired on the page
func (t *Tab) NavigateAndWaitFor(addr string, headers map[string]any, eventName string) error {
	// network.SetBlockedURLS([]string{"*/api/frontend-metrics", "*/api/live/ws", "*/api/user/*"}),
	err := t.Run(enableLifeCycleEvents())
	if err != nil {
		return fmt.Errorf("error enable lifecycle events: %w", err)
	}

	if headers != nil {
		err = t.Run(setHeaders(headers))
		if err != nil {
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

	err = t.Run(waitFor(eventName))
	if err != nil {
		return fmt.Errorf("error waiting for %s on page %s: %w", eventName, addr, err)
	}

	return nil
}

// WithTimeout set the timeout for the actions in the current tab
func (t *Tab) WithTimeout(timeout time.Duration) {
	t.ctx, t.cancel = context.WithTimeout(t.ctx, timeout)
}

// Run executes the actions in the current tab
func (t *Tab) Run(actions ...chromedp.Action) error {
	return chromedp.Run(t.ctx, actions...)
}

// Context returns the current tab's context
func (t *Tab) Context() context.Context {
	return t.ctx
}

// PrintToPDF returns chroms tasks that print the requested HTML into a PDF and returns the PDF stream handle
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
