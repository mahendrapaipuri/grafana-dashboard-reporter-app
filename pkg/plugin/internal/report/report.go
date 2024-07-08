package report

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"math"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/io"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/chrome"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/client"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/config"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/internal/dashboard"
)

// Embed the entire directory.
//
//go:embed templates
var templateFS embed.FS

// Base64 content signatures
var popularSignatures = map[string]string{
	"JVBERi0":     "application/pdf",
	"R0lGODdh":    "image/gif",
	"R0lGODlh":    "image/gif",
	"iVBORw0KGgo": "image/png",
	"/9j/":        "image/jpg",
	"Qk02U":       "image/bmp",
}

// Report groups functions related to genrating the PDF.
type Report interface {
	Generate(ctx context.Context) ([]byte, error)
}

// Options contains Report options
type Options struct {
	DashUID     string
	Layout      string
	Orientation string
	TimeRange   dashboard.TimeRange
}

// Location of time zone
func (o Options) location(timeZone string) *time.Location {
	if location, err := time.LoadLocation(timeZone); err != nil {
		return time.Now().Local().Location()
	} else {
		return location
	}
}

// Data structures used inside HTML template
type templateData struct {
	Options
	Date string

	Dashboard dashboard.Dashboard
	Conf      *config.Config
}

// IsGridLayout returns true if layout config is grid
func (t templateData) IsGridLayout() bool {
	return t.Conf.Layout == "grid"
}

// From returns from time string
func (t templateData) From() string {
	return t.TimeRange.FromFormatted(t.location(t.Conf.TimeZone))
}

// To returns to time string
func (t templateData) To() string {
	return t.TimeRange.ToFormatted(t.location(t.Conf.TimeZone))
}

// Logo returns encoded logo
func (t templateData) Logo() string {
	return t.Conf.EncodedLogo
}

// Panels returns dashboard's panels
func (t templateData) Panels() []dashboard.Panel {
	return t.Dashboard.Panels
}

// Title returns dashboard's title
func (t templateData) Title() string {
	return t.Dashboard.Title
}

// VariableValues returns dashboards query variables
func (t templateData) VariableValues() string {
	return t.Dashboard.VariableValues
}

// PDF represents a PDF report.
type PDF struct {
	chromeInstance chrome.Instance
	conf           *config.Config
	client         client.Grafana
	logger         log.Logger
	options        *Options

	grafanaDashboard dashboard.Dashboard
	pdfOptions       chrome.PDFOptions
}

// New creates a new PDF struct.
func New(logger log.Logger, conf *config.Config, chromeInstance chrome.Instance, client client.Grafana, options *Options) (*PDF, error) {
	return &PDF{
		chromeInstance: chromeInstance,
		conf:           conf,
		client:         client,
		logger:         logger,
		options:        options,
		pdfOptions: chrome.PDFOptions{
			Orientation: options.Orientation,
		},
	}, nil
}

func (r *PDF) fetchDashboard(ctx context.Context) error {
	var err error

	// Get dashboard JSON model
	r.grafanaDashboard, err = r.client.Dashboard(ctx, r.options.DashUID)
	if err != nil {
		return fmt.Errorf("error fetching dashboard %s: %w", r.options.DashUID, err)
	}

	// If we get empty dashboard model, return error
	if reflect.DeepEqual(dashboard.Dashboard{}, r.grafanaDashboard) {
		return fmt.Errorf("empty fetching dashboard %s", r.options.DashUID)
	}

	r.logger.Warn("error(s) fetching dashboard model and data", "err", err, "dash_uid", r.options.DashUID)

	return nil
}

// Generate returns the PDF.pdf file.
// After reading this file, it should be Closed()
// After closing the file, call PDF.Clean() to delete the file as well the temporary build files
func (r *PDF) Generate(ctx context.Context) (io.StreamHandle, error) {
	var err error

	if err = r.fetchDashboard(ctx); err != nil {
		return "", err
	}

	// Render panel PNGs in parallel using max workers configured in plugin
	if err = r.renderPNGsParallel(ctx); err != nil {
		return "", fmt.Errorf("error rendering PNGs in parallel for dashboard %s: %w", r.grafanaDashboard.Title, err)
	}

	// Generate HTML file with fetched panel PNGs
	if err = r.generateHTMLFile(); err != nil {
		return "", fmt.Errorf("error generating HTML file for dashboard %s: %w", r.grafanaDashboard.Title, err)
	}

	// Print HTML page into PDF
	pdfStream, err := r.renderPDF(ctx)
	if err != nil {
		return "", fmt.Errorf("error rendering PDF for dashboard %s: %w", r.grafanaDashboard.Title, err)
	}

	return pdfStream, nil
}

// Title returns the dashboard title parsed from the dashboard definition
func (r *PDF) Title() string {
	return r.grafanaDashboard.Title
}

// renderPNGsParallel renders panel PNGs in parallel using configured number of workers
func (r *PDF) renderPNGsParallel(ctx context.Context) error {
	// buffer all panels on a channel
	panels := make(chan int, len(r.grafanaDashboard.Panels))
	for iPanel := range r.grafanaDashboard.Panels {
		panels <- iPanel
	}
	close(panels)

	// fetch images in parallel form Grafana sever.
	// limit concurrency using a worker pool to avoid overwhelming grafana
	// for dashboards with many panels.
	var wg sync.WaitGroup
	workers := int(math.Max(1, math.Min(float64(r.conf.MaxRenderWorkers), float64(runtime.NumCPU()))))
	wg.Add(workers)
	errs := make(chan error, len(r.grafanaDashboard.Panels)) // routines can return errors on a channel
	for i := 0; i < workers; i++ {
		go func(panels <-chan int, errs chan<- error) {
			defer wg.Done()

			for iPanel := range panels {
				if err := r.renderPNG(ctx, iPanel); err != nil {
					errs <- err
				}
			}
		}(panels, errs)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return fmt.Errorf("error rendering PNG: %w", err)
		}
	}

	return nil
}

// renderPNG renders a single panel into PNG
func (r *PDF) renderPNG(ctx context.Context, iPanel int) error {
	var err error
	r.grafanaDashboard.Panels[iPanel].EncodedImage, err = r.client.PanelPNG(ctx,
		r.options.DashUID,
		r.grafanaDashboard.Panels[iPanel],
		r.options.TimeRange,
	)

	if err != nil {
		return fmt.Errorf("error getting panel %s: %w", r.grafanaDashboard.Panels[iPanel].Title, err)
	}

	return nil
}

// generateHTMLFile generates HTML files for PDF
func (r *PDF) generateHTMLFile() error {
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

	// Make a new template for Body of the PDF
	if tmpl, err = template.New("report").Funcs(funcMap).ParseFS(templateFS, "templates/report.gohtml"); err != nil {
		return fmt.Errorf("error parsing PDF template: %w", err)
	}

	// Template data
	data := templateData{
		*r.options,
		time.Now().Local().In(r.options.location(r.conf.TimeZone)).Format(time.RFC850),
		r.grafanaDashboard,
		r.conf,
	}

	// Render the template for Body of the PDF
	bufBody := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufBody, "report.gohtml", data); err != nil {
		return fmt.Errorf("error executing PDF template: %v", err)
	}
	r.pdfOptions.Body = bufBody.String()
	// r.logger.Debug("Templated HTML Body", "content", truncateBase64Encoding(r.options.html.Body))

	// Make a new template for Header of the PDF
	if tmpl, err = template.New("header").Funcs(funcMap).ParseFS(templateFS, "templates/header.gohtml"); err != nil {
		return fmt.Errorf("error parsing Header template: %w", err)
	}

	// Render the template for Header of the PDF
	bufHeader := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufHeader, "header.gohtml", data); err != nil {
		return fmt.Errorf("error executing Header template: %w", err)
	}
	r.pdfOptions.Header = bufHeader.String()
	// r.logger.Debug("Templated HTML Header", "content", truncateBase64Encoding(r.options.html.Header))

	// Make a new template for Footer of the PDF
	if tmpl, err = template.New("footer").Funcs(funcMap).ParseFS(templateFS, "templates/footer.gohtml"); err != nil {
		return fmt.Errorf("error parsing Footer template: %w", err)
	}

	// Render the template for Footer of the PDF
	bufFooter := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufFooter, "footer.gohtml", data); err != nil {
		return fmt.Errorf("error executing Footer template: %w", err)
	}
	r.pdfOptions.Footer = bufFooter.String()
	// r.logger.Debug("Templated HTML Footer", "content", truncateBase64Encoding(r.options.html.Footer))
	return nil
}

// renderPDF renders HTML page into PDF using Chromium
func (r *PDF) renderPDF(ctx context.Context) (io.StreamHandle, error) {
	// Create a new tab
	tab := r.chromeInstance.NewTab(ctx, r.logger, r.conf)
	defer tab.Close()

	stream, err := tab.PrintToPDF(r.pdfOptions)

	if err != nil {
		return stream, fmt.Errorf("error rendering PDF: %w", err)
	}

	return stream, err
}
