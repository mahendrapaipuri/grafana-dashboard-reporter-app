package plugin

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
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
	ReportOptions
	Images PanelImages
	Date   string
}

// Report options
type ReportOptions struct {
	config     *Config
	dashTitle  string
	dashUID    string
	timeRange  TimeRange
	vfs        *afero.BasePathFs
	reportsDir string
	header     string
	footer     string
}

type PanelImages map[int]template.URL

// Is layout grid?
func (o ReportOptions) IsGridLayout() bool {
	return (o.config.Layout == "grid")
}

// Is orientation landscape?
func (o ReportOptions) IsLandscapeOrientation() bool {
	return (o.config.Orientation == "landscape")
}

// Get from time string
func (o ReportOptions) From() string {
	return o.timeRange.FromFormatted(o.location())
}

// Get to time string
func (o ReportOptions) To() string {
	return o.timeRange.ToFormatted(o.location())
}

// Get logo
func (o ReportOptions) Logo() string {
	return o.config.EncodedLogo
}

// Location of time zone
func (o ReportOptions) location() *time.Location {
	if location, err := time.LoadLocation(o.config.TimeZone); err != nil {
		return time.Now().Local().Location()
	} else {
		return location
	}
}

// report struct
type report struct {
	logger  log.Logger
	client  GrafanaClient
	options *ReportOptions
}

const (
	imgDir     = "images"
	reportHTML = "report.html"
	reportPDF  = "report.pdf"
)

func newReport(logger log.Logger, client GrafanaClient, options *ReportOptions) (*report, error) {
	var err error
	if options.config.PersistData {
		options.reportsDir = filepath.Join("reports", "debug", uuid.New().String())
	} else {
		options.reportsDir = filepath.Join("reports", "production", uuid.New().String())
	}
	if err = options.vfs.MkdirAll(options.reportsDir, 0750); err != nil {
		return nil, err
	}
	return &report{logger, client, options}, nil
}

// New creates a new Report.
func NewReport(logger log.Logger, client GrafanaClient, options *ReportOptions) (Report, error) {
	return newReport(logger, client, options)
}

// Generate returns the report.pdf file.  After reading this file it should be Closed()
// After closing the file, call report.Clean() to delete the file as well the temporary build files
func (r *report) Generate() ([]byte, error) {
	// Get dashboard JSON model
	dash, err := r.client.Dashboard(r.options.dashUID)
	if err != nil {
		// If we get empty dashboard model, return error
		if reflect.DeepEqual(Dashboard{}, dash) {
			return nil, fmt.Errorf("error fetching dashboard %s: %v", r.options.dashUID, err)
		} else {
			r.logger.Warn("error(s) fetching dashboard model and data", "err", err, "dash_title", r.options.dashTitle)
		}
	}
	r.options.dashTitle = dash.Title

	// Render panel PNGs in parallel using max workers configured in plugin
	images, err := r.renderPNGsParallel(dash)
	if err != nil {
		return nil, fmt.Errorf("error rendering PNGs in parallel for dashboard %s: %v", dash.Title, err)
	}

	// Generate HTML file with fetched panel PNGs
	htmlReport, err := r.generateHTMLFile(dash, images)
	if err != nil {
		return nil, fmt.Errorf("error generating HTML file for dashboard %s: %v", dash.Title, err)
	}

	// Print HTML page into PDF
	return r.renderPDF(htmlReport)
}

// Title returns the dashboard title parsed from the dashboard definition
func (r *report) Title() string {
	// lazy fetch if Title() is called before Generate()
	if r.options.dashTitle == "" {
		dash, err := r.client.Dashboard(r.options.dashUID)
		if err != nil {
			return ""
		}
		r.options.dashTitle = dash.Title
	}
	return r.options.dashTitle
}

// Clean deletes the reports directory used during report generation
func (r *report) Clean() {
	err := r.options.vfs.RemoveAll(r.options.reportsDir)
	if err != nil {
		r.logger.Warn("error cleaning up ephermal files", "err", err, "dash_title", r.options.dashTitle)
	}
}

// Get path to images directory
func (r *report) imgDirPath() string {
	return filepath.Join(r.options.reportsDir, imgDir)
}

// Get path to HTML file
func (r *report) htmlPath() string {
	return filepath.Join(r.options.reportsDir, reportHTML)
}

// renderPNGsParallel render panel PNGs in parallel using configured amount workers.
func (r *report) renderPNGsParallel(dash Dashboard) (PanelImages, error) {
	// buffer all panels on a channel
	panels := make(chan Panel, len(dash.Panels))
	for _, p := range dash.Panels {
		panels <- p
	}
	close(panels)

	panelImages := make(PanelImages, len(dash.Panels))

	// fetch images in parallel form Grafana sever.
	// limit concurrency using a worker pool to avoid overwhelming grafana
	// for dashboards with many panels.
	var wg sync.WaitGroup
	workers := int(math.Max(1, math.Min(float64(r.options.config.MaxRenderWorkers), float64(runtime.NumCPU()))))
	wg.Add(workers)
	errs := make(chan error, len(dash.Panels)) // routines can return errors on a channel
	for i := 0; i < workers; i++ {
		go func(panels <-chan Panel, errs chan<- error) {
			defer wg.Done()
			for p := range panels {
				image, err := r.renderPNG(p)
				if err != nil {
					errs <- err
				}

				panelImages[p.ID] = image
			}
		}(panels, errs)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return panelImages, nil
}

// Render a single panel into PNG
func (r *report) renderPNG(p Panel) (template.URL, error) {
	var body io.ReadCloser
	var err error

	// Get panel
	if body, err = r.client.PanelPNG(p, r.options.dashUID, r.options.timeRange); err != nil {
		return "", fmt.Errorf("error getting panel %s: %w", p.Title, err)
	}
	defer body.Close()

	imageContent, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("error reading image content for panel %s: %w", p.Title, err)
	}

	imageContentBase64 := make([]byte, base64.StdEncoding.EncodedLen(len(imageContent)))
	base64.StdEncoding.Encode(imageContentBase64, imageContent)

	return template.URL("data:image/png;charset=utf-8;base64," + string(imageContentBase64)), nil
}

// Generate HTML file(s) for dashboard
func (r *report) generateHTMLFile(dash Dashboard, images PanelImages) (string, error) {
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

	// Make a new template for body of the report
	if tmpl, err = template.New("report").Funcs(funcMap).ParseFS(templateFS, "templates/report.gohtml"); err != nil {
		return "", fmt.Errorf("error parsing report template: %w", err)
	}

	// Template data
	data := templateData{dash, *r.options, images, time.Now().Local().In(r.options.location()).Format(time.RFC850)}

	// Render the template for body of the report
	bufReport := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufReport, "report.gohtml", data); err != nil {
		return "", fmt.Errorf("error executing report template: %w", err)
	}

	// Make a new template for header of the report
	if tmpl, err = template.New("header").Funcs(funcMap).ParseFS(templateFS, "templates/header.gohtml"); err != nil {
		return "", fmt.Errorf("error parsing header template: %w", err)
	}

	// Render the template for header of the report
	bufHeader := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufHeader, "header.gohtml", data); err != nil {
		return "", fmt.Errorf("error executing header template: %w", err)
	}
	r.options.header = bufHeader.String()

	// Make a new template for footer of the report
	if tmpl, err = template.New("footer").Funcs(funcMap).ParseFS(templateFS, "templates/footer.gohtml"); err != nil {
		return "", fmt.Errorf("error parsing footer template: %w", err)
	}

	// Render the template for footer of the report
	bufFooter := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufFooter, "footer.gohtml", data); err != nil {
		return "", fmt.Errorf("error executing footer template: %w", err)
	}
	r.options.footer = bufFooter.String()

	return bufReport.String(), nil
}

// Render HTML page into PDF using Chromium
func (r *report) renderPDF(htmlReport string) ([]byte, error) {
	var realPath string
	var err error

	// Get real path on actual file system
	if realPath, err = r.options.vfs.RealPath(r.options.reportsDir); err != nil {
		return nil, err
	}

	// create context
	allocCtx, allocCtxCancel := chromedp.NewExecAllocator(context.Background(), r.options.config.ChromeOptions...)
	defer allocCtxCancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// capture pdf
	var buf []byte
	if err = chromedp.Run(
		ctx, r.printToPDF(htmlReport, &buf),
	); err != nil {
		return nil, fmt.Errorf("error rendering PDF: %v", err)
	}

	// If persistData is set to true, write buf to file
	if r.options.config.PersistData {
		if err := os.WriteFile(filepath.Join(realPath, reportPDF), buf, 0o640); err != nil {
			return nil, fmt.Errorf("error writing PDF: %v", err)
		}
	}
	return buf, err
}

// Print to PDF using headless Chromium
func (r *report) printToPDF(html string, res *[]byte) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}

			return page.SetDocumentContent(frameTree.Frame.ID, html).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {

			var pageParams *page.PrintToPDFParams
			// In CI mode do not add header and footer for visual comparison
			if os.Getenv("__REPORTER_APP_CI_MODE") == "true" {
				pageParams = page.PrintToPDF().
					WithPreferCSSPageSize(true)
			} else {
				pageParams = page.PrintToPDF().
					WithDisplayHeaderFooter(true).
					WithHeaderTemplate(r.options.header).
					WithFooterTemplate(r.options.footer).
					WithPreferCSSPageSize(true)
			}

			// If landscape add it to page params
			if r.options.IsLandscapeOrientation() {
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
