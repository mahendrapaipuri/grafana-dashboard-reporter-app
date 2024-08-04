package chrome

import (
	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
	"golang.org/x/net/context"
)

type RemoteInstance struct {
	allocCtx       context.Context
	allocCtxCancel context.CancelFunc
}

// NewRemoteBrowserInstance creates a new remote browser instance
func NewRemoteBrowserInstance(ctx context.Context, _ log.Logger, remoteChromeURL string) (*RemoteInstance, error) {
	allocCtx, allocCtxCancel := chromedp.NewRemoteAllocator(ctx, remoteChromeURL)

	return &RemoteInstance{allocCtx, allocCtxCancel}, nil
}

func (i *RemoteInstance) Name() string {
	return "remote"
}

func (i *RemoteInstance) NewTab(logger log.Logger, conf *config.Config) *Tab {
	chromeLogger := logger.With("subsystem", "chromium")
	browserCtx, _ := chromedp.NewContext(i.allocCtx,
		chromedp.WithErrorf(chromeLogger.Error),
		chromedp.WithLogf(chromeLogger.Debug),
	)

	return &Tab{
		ctx: browserCtx,
	}
}

func (i *RemoteInstance) Close(_ log.Logger) {
	if i.allocCtxCancel != nil {
		i.allocCtxCancel()
	}
}
