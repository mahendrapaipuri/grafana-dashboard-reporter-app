package plugin

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/spf13/afero"
)

// Embed the entire directory.
//
//go:embed templates
var templateFS embed.FS

// Report groups functions related to genrating the report.
// After reading and closing the pdf returned by Generate(), call Clean() to delete the pdf file as well the temporary build files
type Report interface {
	Generate() ([]byte, error)
	Title() string
	Clean()
}

// Data structures used inside HTML template
type templateData struct {
	Dashboard
	ReportConfig
	Date string
}

// Report config
type ReportConfig struct {
	dashTitle        string
	dashUID          string
	timeRange        TimeRange
	vfs              *afero.BasePathFs
	stagingDir       string
	maxRenderWorkers int
	layout           string
	orientation      string
	persistData      bool
	header           string
	footer           string
}

// Is layout grid?
func (c ReportConfig) IsGridLayout() bool {
	return (c.layout == "grid")
}

// Is orientation landscape?
func (c ReportConfig) IsLandscapeOrientation() bool {
	return (c.orientation == "landscape")
}

// Get from time string
func (c ReportConfig) From() string {
	return c.timeRange.FromFormatted()
}

// Get to time string
func (c ReportConfig) To() string {
	return c.timeRange.ToFormatted()
}

// report struct
type report struct {
	logger log.Logger
	client GrafanaClient
	cfg    *ReportConfig
}

const (
	imgDir     = "images"
	reportHTML = "report.html"
	reportPDF  = "report.pdf"
)

func newReport(logger log.Logger, client GrafanaClient, config *ReportConfig) (*report, error) {
	var err error
	if config.persistData {
		config.stagingDir = filepath.Join("staging", "debug", uuid.New().String())
	} else {
		config.stagingDir = filepath.Join("staging", "production", uuid.New().String())
	}
	if err = config.vfs.MkdirAll(config.stagingDir, 0750); err != nil {
		return nil, err
	}
	return &report{logger, client, config}, nil
}

// New creates a new Report.
func NewReport(logger log.Logger, client GrafanaClient, config *ReportConfig) (Report, error) {
	return newReport(logger, client, config)
}

// Generate returns the report.pdf file.  After reading this file it should be Closed()
// After closing the file, call report.Clean() to delete the file as well the temporary build files
func (r *report) Generate() ([]byte, error) {
	// Get dashboard JSON model
	dash, err := r.client.GetDashboard(r.cfg.dashUID)
	if err != nil {
		return nil, fmt.Errorf("error fetching dashboard %s: %v", r.cfg.dashUID, err)
	}
	r.cfg.dashTitle = dash.Title

	// Render panel PNGs in parallel using max workers configured in plugin
	if err = r.renderPNGsParallel(dash); err != nil {
		return nil, fmt.Errorf("error rendering PNGs in parallel for dashboard %s: %v", dash.Title, err)
	}

	// Generate HTML file with fetched panel PNGs
	if err = r.generateHTMLFile(dash); err != nil {
		return nil, fmt.Errorf("error generating HTML file for dashboard %s: %v", dash.Title, err)
	}

	// Print HTML page into PDF
	return r.renderPDF()
}

// Title returns the dashboard title parsed from the dashboard definition
func (r *report) Title() string {
	// lazy fetch if Title() is called before Generate()
	if r.cfg.dashTitle == "" {
		dash, err := r.client.GetDashboard(r.cfg.dashUID)
		if err != nil {
			return ""
		}
		r.cfg.dashTitle = dash.Title
	}
	return r.cfg.dashTitle
}

// Clean deletes the staging directory used during report generation
func (r *report) Clean() {
	err := r.cfg.vfs.RemoveAll(r.cfg.stagingDir)
	if err != nil {
		r.logger.Warn("error cleaning up staging dir", "err", err, "dashTitle", r.cfg.dashTitle)
	}
}

// Get path to images directory
func (r *report) imgDirPath() string {
	return filepath.Join(r.cfg.stagingDir, imgDir)
}

// Get path to HTML file
func (r *report) htmlPath() string {
	return filepath.Join(r.cfg.stagingDir, reportHTML)
}

// Render panel PNGs in parallel using configured number of workers
func (r *report) renderPNGsParallel(dash Dashboard) error {
	// buffer all panels on a channel
	panels := make(chan Panel, len(dash.Panels))
	for _, p := range dash.Panels {
		panels <- p
	}
	close(panels)

	// fetch images in parrallel form Grafana sever.
	// limit concurrency using a worker pool to avoid overwhelming grafana
	// for dashboards with many panels.
	var wg sync.WaitGroup
	workers := int(math.Max(1, math.Min(float64(r.cfg.maxRenderWorkers), float64(runtime.NumCPU()))))
	wg.Add(workers)
	errs := make(chan error, len(dash.Panels)) // routines can return errors on a channel
	for i := 0; i < workers; i++ {
		go func(panels <-chan Panel, errs chan<- error) {
			defer wg.Done()
			for p := range panels {
				err := r.renderPNG(p)
				if err != nil {
					errs <- err
				}
			}
		}(panels, errs)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// Render a single panel into PNG
func (r *report) renderPNG(p Panel) error {
	var body io.ReadCloser
	var file afero.File
	var err error

	// Get panel
	if body, err = r.client.GetPanelPNG(p, r.cfg.dashUID, r.cfg.timeRange); err != nil {
		return fmt.Errorf("error getting panel %s: %v", p.Title, err)
	}
	defer body.Close()

	// Create directory to store PNG files and get file handler
	if err = r.cfg.vfs.MkdirAll(r.imgDirPath(), 0750); err != nil {
		return fmt.Errorf("error creating img directory: %v", err)
	}
	imgFileName := fmt.Sprintf("image%d.png", p.Id)
	if file, err = r.cfg.vfs.Create(filepath.Join(r.imgDirPath(), imgFileName)); err != nil {
		return fmt.Errorf("error creating image file: %v", err)
	}
	defer file.Close()

	// Copy PNG to file
	if _, err = io.Copy(file, body); err != nil {
		return fmt.Errorf("error copying body to file: %v", err)
	}
	return nil
}

// Generate HTML file(s) for dashboard
func (r *report) generateHTMLFile(dash Dashboard) error {
	var file afero.File
	var tmpl *template.Template
	var err error

	// Template functions
	funcMap := template.FuncMap{
		// The name "inc" is what the function will be called in the template text.
		"inc": func(i float64) float64 {
			return i + 1
		},

		"mult": func(i int) int {
			return i*30 + 5
		},
	}

	// Make a file handle for HTML file
	if file, err = r.cfg.vfs.Create(r.htmlPath()); err != nil {
		return fmt.Errorf("error creating HTML file at %v : %v", r.htmlPath(), err)
	}
	defer file.Close()

	// Make a new template for body of the report
	if tmpl, err = template.New("report").Funcs(funcMap).ParseFS(templateFS, "templates/report.gohtml"); err != nil {
		return fmt.Errorf("error parsing report template: %v", err)
	}

	// Render the template for body of the report
	if err = tmpl.ExecuteTemplate(
		file,
		"report.gohtml",
		templateData{dash, *r.cfg, time.Now().Format(time.RFC850)}); err != nil {
		return fmt.Errorf("error executing report template: %v", err)
	}

	// Make a new template for header of the report
	if tmpl, err = template.New("header").Funcs(funcMap).ParseFS(templateFS, "templates/header.gohtml"); err != nil {
		return fmt.Errorf("error parsing header template: %v", err)
	}

	// Render the template for header of the report
	bufHeader := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(
		bufHeader,
		"header.gohtml",
		templateData{dash, *r.cfg, time.Now().Format(time.RFC850)}); err != nil {
		return fmt.Errorf("error executing header template: %v", err)
	}
	r.cfg.header = bufHeader.String()

	// Make a new template for footer of the report
	if tmpl, err = template.New("footer").Funcs(funcMap).ParseFS(templateFS, "templates/footer.gohtml"); err != nil {
		return fmt.Errorf("error parsing footer template: %v", err)
	}

	// Render the template for footer of the report
	bufFooter := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(
		bufFooter,
		"footer.gohtml",
		templateData{dash, *r.cfg, time.Now().Format(time.RFC850)}); err != nil {
		return fmt.Errorf("error executing footer template: %v", err)
	}
	r.cfg.footer = bufFooter.String()
	return nil
}

// Render HTML page into PDF using Chromium
func (r *report) renderPDF() ([]byte, error) {
	var realPath string
	var err error

	// Get real path on actual file system
	if realPath, err = r.cfg.vfs.RealPath(r.cfg.stagingDir); err != nil {
		return nil, err
	}

	// Chrome executor options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
	)
	
	// create context
	allocCtx, allocCtxCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCtxCancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// capture pdf
	var buf []byte
	if err := chromedp.Run(
		ctx, r.printToPDF(fmt.Sprintf("file://%s", filepath.Join(realPath, reportHTML)), &buf),
	); err != nil {
		return nil, fmt.Errorf("error rendering PDF: %v", err)
	}

	// If persistData is set to true, write buf to file
	if r.cfg.persistData {
		if err := os.WriteFile(filepath.Join(realPath, reportPDF), buf, 0o640); err != nil {
			return nil, fmt.Errorf("error writing PDF: %v", err)
		}
	}
	return buf, err
}

// Print to PDF using headless Chromium
func (r *report) printToPDF(url string, res *[]byte) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			pageParams := page.PrintToPDF().
				WithDisplayHeaderFooter(true).
				WithHeaderTemplate(r.cfg.header).
				WithFooterTemplate(r.cfg.footer).
				WithPreferCSSPageSize(true)

			// If landscape add it to page params
			if r.cfg.IsLandscapeOrientation() {
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
