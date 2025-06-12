package chrome

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"golang.org/x/net/context"
)

// Path to chrome executable.
var (
	chromeExec    string
	chromeHomeDir string
)

func init() {
	// Seems like using the same headless chrome distibution from grafana-image-renderer
	// does not work on Windows. Until we ship our own distribution of headless chrome,
	// skip this part.
	if runtime.GOOS == "windows" {
		return
	}

	// Get Grafana data path based on path of current executable
	pluginExe, err := os.Executable()
	if err != nil {
		panic(err)
	}

	// Generally this pluginExe should be at install_dir/plugins/mahendrapaipuri-dashboardreporter-app/exe
	// Now we attempt to get install_dir directory which is Grafana data path
	dataPath := filepath.Dir(filepath.Dir(filepath.Dir(pluginExe)))

	// Create a folder to use it as HOME for chrome process
	homeDir := filepath.Join(dataPath, ".chrome")
	if err := os.MkdirAll(homeDir, 0o750); err == nil {
		chromeHomeDir = homeDir
	}

	// Walk through grafana-image-renderer plugin dir to find chrome executable
	_ = filepath.Walk(filepath.Join(dataPath, "plugins", "grafana-image-renderer"),
		func(path string, info fs.FileInfo, err error) error {
			// prevent panic by handling failure accessing a path
			if err != nil {
				return err
			}

			// In recent releases of grafana-image-renderer, the binary is called chrome-headless-shell
			validChromeBins := []string{"chrome", "chrome.exe", "chrome-headless-shell", "chrome-headless-shell.exe"}
			if !info.IsDir() && slices.Contains(validChromeBins, info.Name()) {
				// If the chrome shipped is not "usable", plugin cannot be used
				// even a "usable" chrome (for instance chromium installed using snap on Ubuntu)
				// exists.
				// So, test the chromium before using it
				//
				// Use a timeout to avoid indefinite hanging
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				// This command should print an empty DOM and exit
				if _, err := exec.CommandContext(
					ctx, path, "--headless", "--no-sandbox", "--disable-gpu",
					"--disable-logging ", "--dump-dom",
				).Output(); err == nil {
					chromeExec = path
				}

				return nil
			}

			return nil
		},
	)
}

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

	// If we managed to create a home for chrome in a "writable" location, set it to chrome options
	if chromeHomeDir != "" {
		logger.Debug("created home directory for chromium process", "home", chromeHomeDir)

		// Seems like on windows using headless chrome distributed by grafana-image-renderer
		// produces a debug log of chrome in the same folder which violates the list of
		// files distributed in the MANIFEST and hence, Grafana refuses to run grafana-image-renderer.
		// So, override default chrome's --log-file location to the new home that we created for
		// chrome.
		chromeOptions = append(
			chromeOptions, chromedp.Env(
				"XDG_CONFIG_HOME="+chromeHomeDir, "XDG_CACHE_HOME="+chromeHomeDir,
				"CHROME_LOG_FILE="+filepath.Join(chromeHomeDir, "debug.log"),
			),
		)

		// If we managed to make chrome home dir and find chrom exec from `grafana-image-renderer` use it.
		if chromeExec != "" {
			logger.Info("chrome executable provided by grafana-image-renderer will be used", "chrome", chromeExec)
			chromeOptions = append(chromeOptions, chromedp.ExecPath(chromeExec))
		}
	}

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
func (i *LocalInstance) NewTab(_ log.Logger, _ *config.Config) *Tab {
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
}
