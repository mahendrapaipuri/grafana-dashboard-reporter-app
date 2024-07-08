package chrome

import (
	"fmt"

	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/config"
	"golang.org/x/net/context"
)

type LocalInstance struct {
	browserCtx context.Context

	browserCtxCancel context.CancelFunc
	allocCtxCancel   context.CancelFunc
}

// NewLocalBrowserInstance creates a new local browser instance
func NewLocalBrowserInstance(ctx context.Context, logger log.Logger, insecureSkipVerify bool) (*LocalInstance, error) {
	// preallocate options
	chromeOptions := make([]chromedp.ExecAllocatorOption, 0, len(chromedp.DefaultExecAllocatorOptions)+3)

	// Set chrome options
	chromeOptions = append(chromedp.DefaultExecAllocatorOptions[:],
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
	allocCtx, allocCtxCancel := chromedp.NewExecAllocator(ctx, chromeOptions...)

	// start a browser (and an empty tab) so we can add more tabs to the browser
	chromeLogger := logger.With("subsystem", "chromium")
	browserCtx, browserCtxCancel := chromedp.NewContext(allocCtx,
		chromedp.WithErrorf(chromeLogger.Error),
		chromedp.WithLogf(chromeLogger.Debug),
	)

	if err := chromedp.Run(browserCtx); err != nil {
		return nil, fmt.Errorf("couldn't create browser context: %w", err)
	}

	return &LocalInstance{
		browserCtx,
		browserCtxCancel,
		allocCtxCancel,
	}, nil
}

func (i *LocalInstance) NewTab(ctx context.Context, _ log.Logger, _ *config.Config) *Tab {
	// start a browser (and an empty tab) so we can add more tabs to the browser
	ctx, cancel := chromedp.NewContext(i.browserCtx)

	return &Tab{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (i *LocalInstance) Close() {
	if i.browserCtxCancel != nil {
		i.browserCtxCancel()
	}

	if i.allocCtxCancel != nil {
		i.allocCtxCancel()
	}
}
