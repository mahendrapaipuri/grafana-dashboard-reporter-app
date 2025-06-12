package chrome

import (
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"golang.org/x/net/context"
)

// RemoteInstance is a remotely running browser instance.
type RemoteInstance struct {
	allocCtx       context.Context
	allocCtxCancel context.CancelFunc
}

// NewRemoteBrowserInstance creates a new remote browser instance.
func NewRemoteBrowserInstance(ctx context.Context, _ log.Logger, remoteChromeURL string) (*RemoteInstance, error) {
	allocCtx, allocCtxCancel := chromedp.NewRemoteAllocator(ctx, remoteChromeURL)

	return &RemoteInstance{allocCtx, allocCtxCancel}, nil
}

// Name returns the kind of browser instance.
func (i *RemoteInstance) Name() string {
	return "remote"
}

// NewTab starts and returns a new tab on current browser instance.
func (i *RemoteInstance) NewTab(logger log.Logger, _ *config.Config) *Tab {
	chromeLogger := logger.With("subsystem", "chromium")
	browserCtx, _ := chromedp.NewContext(i.allocCtx,
		chromedp.WithErrorf(chromeLogger.Error),
		chromedp.WithLogf(chromeLogger.Debug),
	)

	return &Tab{
		ctx: browserCtx,
	}
}

// Close releases the resources of browser instance.
func (i *RemoteInstance) Close(_ log.Logger) {
	if i.allocCtxCancel != nil {
		i.allocCtxCancel()
	}
}
