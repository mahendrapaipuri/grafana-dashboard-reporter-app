package chrome

import (
	"fmt"
	"os"

	"github.com/chromedp/cdproto/io"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"golang.org/x/net/context"
)

type Tab struct {
	parentCtxCancel context.CancelFunc
	ctx             context.Context
	cancel          context.CancelFunc
}

func (t *Tab) Close() {
	if t.cancel != nil {
		t.cancel()
	}

	if t.parentCtxCancel != nil {
		t.cancel()
	}
}

func (t *Tab) Run(actions chromedp.Action) error {
	return chromedp.Run(t.ctx, actions)
}

func (t *Tab) Context() context.Context {
	return t.ctx
}

// PrintToPDF returns chroms tasks that print the requested HTML into a PDF and returns the PDF stream handle
func (t *Tab) PrintToPDF(options PDFOptions) (io.StreamHandle, error) {
	var stream io.StreamHandle

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
			_, stream, err = pageParams.Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to print to PDF: %w", err)
			}

			return nil
		}),
	})

	if err != nil {
		return stream, fmt.Errorf("error rendering PDF: %w", err)
	}

	return stream, nil
}
