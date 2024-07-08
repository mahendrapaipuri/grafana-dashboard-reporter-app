package chrome

import (
	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/config"
	"golang.org/x/net/context"
)

type RemoteInstance struct {
	browserCtx context.Context

	browserCtxCancel context.CancelFunc
	allocCtxCancel   context.CancelFunc
}

// NewRemoteBrowserInstance creates a new remote browser instance
func NewRemoteBrowserInstance(_ context.Context, _ log.Logger, _ bool) (*RemoteInstance, error) {
	// we don't need to do anything here. We are connecting to a remote browser if needed.
	return &RemoteInstance{}, nil
}

func (i *RemoteInstance) NewTab(ctx context.Context, logger log.Logger, conf *config.Config) *Tab {
	allocCtx, allocCtxCancel := chromedp.NewRemoteAllocator(ctx, conf.RemoteChromeAddr)

	chromeLogger := logger.With("subsystem", "chromium")
	browserCtx, browserCtxCancel := chromedp.NewContext(allocCtx,
		chromedp.WithErrorf(chromeLogger.Error),
		chromedp.WithLogf(chromeLogger.Debug),
	)

	return &Tab{
		parentCtxCancel: allocCtxCancel,
		ctx:             browserCtx,
		cancel:          browserCtxCancel,
	}
}

func (i *RemoteInstance) Close() {}
