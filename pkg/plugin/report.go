package plugin

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"math"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Embed the entire directory.
//
//go:embed templates
var templateFS embed.FS

// var (
// 	// Regex that will base64 encoded string in HTML data:%s;base64,
// 	regexBase64 = regexp.MustCompile("(?:.+?)data:(?P<mime>((image|application)/(pdf|gif|png|jpg|bmp)));base64,(?P<encoding>[a-zA-Z0-9]+)")
// )

// Base64 content signatures
var popularSignatures = map[string]string{
	"JVBERi0":     "application/pdf",
	"R0lGODdh":    "image/gif",
	"R0lGODlh":    "image/gif",
	"iVBORw0KGgo": "image/png",
	"/9j/":        "image/jpg",
	"Qk02U":       "image/bmp",
}

// Report groups functions related to genrating the report.
type Report interface {
	Generate(ctx context.Context) ([]byte, error)
	Title(ctx context.Context) string
}

// HTMLContent contains the templated HTML body, header and footer strings
type HTMLContent struct {
	body   string
	header string
	footer string
}

// ReportOptions contains report options
type ReportOptions struct {
	config    *Config
	dashboard Dashboard
	dashUID   string
	timeRange TimeRange
	html      HTMLContent
}

// IsLandscapeOrientation returns true if orientation config is landscape
func (o ReportOptions) IsLandscapeOrientation() bool {
	return (o.config.Orientation == "landscape")
}

// Location of time zone
func (o ReportOptions) location() *time.Location {
	if location, err := time.LoadLocation(o.config.TimeZone); err != nil {
		return time.Now().Local().Location()
	} else {
		return location
	}
}

// Data structures used inside HTML template
type templateData struct {
	ReportOptions
	Date string
}

// IsLandscapeOrientation returns true if layout config is grid
func (t templateData) IsGridLayout() bool {
	return (t.config.Layout == "grid")
}

// From returns from time string
func (t templateData) From() string {
	return t.timeRange.FromFormatted(t.location())
}

// To returns to time string
func (t templateData) To() string {
	return t.timeRange.ToFormatted(t.location())
}

// Logo returns encoded logo
func (t templateData) Logo() string {
	return t.config.EncodedLogo
}

// Panels returns dashboard's panels
func (t templateData) Panels() []Panel {
	return t.dashboard.Panels
}

// Title returns dashboard's title
func (t templateData) Title() string {
	return t.dashboard.Title
}

// VariableValues returns dashboards query variables
func (t templateData) VariableValues() string {
	return t.dashboard.VariableValues
}

// report struct
type report struct {
	logger  log.Logger
	client  GrafanaClient
	options *ReportOptions
}

// newReport returns a new instance of report struct
func newReport(logger log.Logger, client GrafanaClient, options *ReportOptions) (*report, error) {
	return &report{logger, client, options}, nil
}

// NewReport creates a new report struct.
func NewReport(logger log.Logger, client GrafanaClient, options *ReportOptions) (Report, error) {
	return newReport(logger, client, options)
}

// Generate returns the report.pdf file.  After reading this file it should be Closed()
// After closing the file, call report.Clean() to delete the file as well the temporary build files
func (r *report) Generate(ctx context.Context) ([]byte, error) {
	var err error
	// Get dashboard JSON model
	r.options.dashboard, err = r.client.Dashboard(ctx, r.options.dashUID)
	if err != nil {
		// If we get empty dashboard model, return error
		if reflect.DeepEqual(Dashboard{}, r.options.dashboard) {
			return nil, fmt.Errorf("error fetching dashboard %s: %v", r.options.dashUID, err)
		} else {
			r.logger.Warn("error(s) fetching dashboard model and data", "err", err, "dash_uid", r.options.dashUID)
		}
	}

	// Render panel PNGs in parallel using max workers configured in plugin
	if err = r.renderPNGsParallel(); err != nil {
		return nil, fmt.Errorf("error rendering PNGs in parallel for dashboard %s: %v", r.options.dashboard.Title, err)
	}

	// Generate HTML file with fetched panel PNGs
	if err = r.generateHTMLFile(); err != nil {
		return nil, fmt.Errorf("error generating HTML file for dashboard %s: %v", r.options.dashboard.Title, err)
	}

	// Print HTML page into PDF
	return r.renderPDF(ctx)
}

// Title returns the dashboard title parsed from the dashboard definition
func (r *report) Title() string {
	return r.options.dashboard.Title
}

// renderPNGsParallel renders panel PNGs in parallel using configured number of workers
func (r *report) renderPNGsParallel() error {
	// buffer all panels on a channel
	panels := make(chan int, len(r.options.dashboard.Panels))
	for iPanel := range r.options.dashboard.Panels {
		panels <- iPanel
	}
	close(panels)

	// fetch images in parallel form Grafana sever.
	// limit concurrency using a worker pool to avoid overwhelming grafana
	// for dashboards with many panels.
	var wg sync.WaitGroup
	workers := int(math.Max(1, math.Min(float64(r.options.config.MaxRenderWorkers), float64(runtime.NumCPU()))))
	wg.Add(workers)
	errs := make(chan error, len(r.options.dashboard.Panels)) // routines can return errors on a channel
	for i := 0; i < workers; i++ {
		go func(panels <-chan int, errs chan<- error) {
			defer wg.Done()
			for iPanel := range panels {
				err := r.renderPNG(iPanel)
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

// renderPNG renders a single panel into PNG
func (r *report) renderPNG(iPanel int) error {
	var err error
	if r.options.dashboard.Panels[iPanel].EncodedImage, err = r.client.PanelPNG(
		r.options.dashboard.Panels[iPanel], r.options.dashUID, r.options.timeRange,
	); err != nil {
		return fmt.Errorf("error getting panel %s: %v", r.options.dashboard.Panels[iPanel].Title, err)
	}
	return nil
}

// generateHTMLFile generates HTML files for report
func (r *report) generateHTMLFile() error {
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

		"embed": func(base64Content string) template.URL {
			for signature, mimeType := range popularSignatures {
				if strings.HasPrefix(base64Content, signature) {
					return template.URL(fmt.Sprintf("data:%s;base64,%s", mimeType, base64Content))
				}
			}
			return template.URL(base64Content)
		},
	}

	// Make a new template for body of the report
	if tmpl, err = template.New("report").Funcs(funcMap).ParseFS(templateFS, "templates/report.gohtml"); err != nil {
		return fmt.Errorf("error parsing report template: %w", err)
	}

	// Template data
	data := templateData{*r.options, time.Now().Local().In(r.options.location()).Format(time.RFC850)}

	// Render the template for body of the report
	bufBody := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufBody, "report.gohtml", data); err != nil {
		return fmt.Errorf("error executing report template: %v", err)
	}
	r.options.html.body = bufBody.String()
	// r.logger.Debug("Templated HTML body", "content", truncateBase64Encoding(r.options.html.body))

	// Make a new template for header of the report
	if tmpl, err = template.New("header").Funcs(funcMap).ParseFS(templateFS, "templates/header.gohtml"); err != nil {
		return fmt.Errorf("error parsing header template: %w", err)
	}

	// Render the template for header of the report
	bufHeader := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufHeader, "header.gohtml", data); err != nil {
		return fmt.Errorf("error executing header template: %w", err)
	}
	r.options.html.header = bufHeader.String()
	// r.logger.Debug("Templated HTML header", "content", truncateBase64Encoding(r.options.html.header))

	// Make a new template for footer of the report
	if tmpl, err = template.New("footer").Funcs(funcMap).ParseFS(templateFS, "templates/footer.gohtml"); err != nil {
		return fmt.Errorf("error parsing footer template: %w", err)
	}

	// Render the template for footer of the report
	bufFooter := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufFooter, "footer.gohtml", data); err != nil {
		return fmt.Errorf("error executing footer template: %w", err)
	}
	r.options.html.footer = bufFooter.String()
	// r.logger.Debug("Templated HTML footer", "content", truncateBase64Encoding(r.options.html.footer))
	return nil
}

// renderPDF renders HTML page into PDF using Chromium
func (r *report) renderPDF(ctx context.Context) ([]byte, error) {
	// var realPath string
	var err error

	chromeLogger := r.logger.With("subsystem", "chromium")

	// Create a new tab
	ctx, cancel := chromedp.NewContext(r.options.config.BrowserContext,
		chromedp.WithErrorf(chromeLogger.Error),
		chromedp.WithDebugf(chromeLogger.Debug),
		chromedp.WithLogf(chromeLogger.Info),
	)
	defer cancel()

	// capture pdf
	var buf []byte
	if err = chromedp.Run(
		ctx,
		printToPDF(r.options.html, r.options.IsLandscapeOrientation(),
			&buf,
		)); err != nil {
		return nil, fmt.Errorf("error rendering PDF: %v", err)
	}
	return buf, err
}

// // truncateBase64Encoding replaces base64 encodings with truncated encodings.
// // Seems like logger has issues logging this properly.
// func truncateBase64Encoding(input string) string {
// 	var encoding string
// 	var truncatedInput = input
// 	matches := regexBase64.FindAllStringSubmatch(input, -1)
// 	for _, match := range matches {
// 		if len(match) > 1 {
// 			encoding = match[len(match)-1]
// 			truncatedInput = strings.Replace(truncatedInput, encoding, encoding[:10]+"[truncated]", -1)
// 		}
// 	}
// 	return truncatedInput
// }
