package chrome

import (
	"fmt"

	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
	"golang.org/x/net/context"
)

// LocalInstance is a locally running browser instance.
type LocalInstance struct {
	allocCtx   context.Context
	browserCtx context.Context
}

// NewLocalBrowserInstance creates a new local browser instance.
func NewLocalBrowserInstance(ctx context.Context, logger log.Logger, insecureSkipVerify bool) (*LocalInstance, error) {
	// go-staticcheck was keep complaining about unused var
	// preallocate options
	// chromeOptions := make([]func(*chromedp.ExecAllocator), 0, len(chromedp.DefaultExecAllocatorOptions)+3)
	// Set chrome options
	chromeOptions := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
	)

	if insecureSkipVerify {
		// Seems like this is critical. When it is not turned on there are no errors
		// and plugin will exit without rendering any panels. Not sure why the error
		// handling is failing here. So, add this option as default just to avoid
		// those cases
		//
		// Ref: https://github.com/chromedp/chromedp/issues/492#issuecomment-543223861
		chromeOptions = append(chromeOptions, chromedp.Flag("ignore-certificate-errors", "1"))
	}

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
	allocCtx, _ := chromedp.NewExecAllocator(ctx, chromeOptions...)

	// start a browser (and an empty tab) so we can add more tabs to the browser
	chromeLogger := logger.With("subsystem", "chromium")
	browserCtx, _ := chromedp.NewContext(allocCtx,
		chromedp.WithErrorf(chromeLogger.Error),
		chromedp.WithLogf(chromeLogger.Debug),
	)

	if err := chromedp.Run(browserCtx); err != nil {
		return nil, fmt.Errorf("couldn't create browser context: %w", err)
	}

	return &LocalInstance{
		allocCtx,
		browserCtx,
	}, nil
}

// Name returns the kind of browser instance.
func (i *LocalInstance) Name() string {
	return "local"
}

// NewTab starts and returns a new tab on current browser instance.
func (i *LocalInstance) NewTab(_ log.Logger, _ config.Config) *Tab {
	ctx, _ := chromedp.NewContext(i.browserCtx)

	return &Tab{
		ctx: ctx,
	}
}

func (i *LocalInstance) Close(logger log.Logger) {
	if i.browserCtx != nil {
		if err := chromedp.Cancel(i.browserCtx); err != nil {
			logger.Error("got error from cancel browser context", "error", err)
		}
	}

	if i.allocCtx != nil {
		if err := chromedp.Cancel(i.allocCtx); err != nil {
			logger.Error("got error from cancel browser allocator context", "error", err)
		}
	}
}
