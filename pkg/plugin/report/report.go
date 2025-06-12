package report

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/helpers"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/worker"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Embed the entire directory.
//
//go:embed templates
var templateFS embed.FS

// Base64 content signatures.
var popularSignatures = map[string]string{
	"JVBERi0":     "application/pdf",
	"R0lGODdh":    "image/gif",
	"R0lGODlh":    "image/gif",
	"iVBORw0KGgo": "image/png",
	"/9j/":        "image/jpg",
	"Qk02U":       "image/bmp",
}

func New(logger log.Logger, conf *config.Config, httpClient *http.Client, chromeInstance chrome.Instance,
	pools worker.Pools, dashboard *dashboard.Dashboard,
) *Report {
	return &Report{
		logger,
		conf,
		httpClient,
		chromeInstance,
		pools,
		dashboard,
	}
}

func (r *Report) Generate(ctx context.Context, writer http.ResponseWriter) error {
	defer helpers.TimeTrack(time.Now(), "report generation", r.logger)

	// Get panel data from dashboard
	dashboardData, err := r.dashboard.GetData(ctx)
	if err != nil {
		return fmt.Errorf("failed to get dashboard data: %w", err)
	}

	// Populate panels with PNG and tabular data
	if err := r.populatePanels(ctx, dashboardData); err != nil {
		return fmt.Errorf("failed to populate panels: %w", err)
	}

	// panelTables = slices.DeleteFunc(panelTables, func(panelTable dashboard.PanelTable) bool {
	// 	return panelTable.Data == nil
	// })

	// Sanitize title to escape non ASCII characters
	// Ref: https://stackoverflow.com/questions/62705546/unicode-characters-in-attachment-name
	// Ref: https://medium.com/@JeremyLaine/non-ascii-content-disposition-header-in-django-3a20acc05f0d
	filename := url.PathEscape(dashboardData.Title)
	header := fmt.Sprintf(`inline; filename*=UTF-8''%s.pdf`, filename)
	writer.Header().Add("Content-Disposition", header)

	htmlReport, err := r.generateHTMLFile(dashboardData)
	if err != nil {
		return fmt.Errorf("failed to generate HTML file: %w", err)
	}

	if err = r.renderPDF(htmlReport, writer); err != nil {
		return fmt.Errorf("failed to render PDF: %w", err)
	}

	return nil
}

// populatePanels populates the panels with PNG and tabular data.
func (r *Report) populatePanels(ctx context.Context, dashboardData *dashboard.Data) error {
	defer helpers.TimeTrack(time.Now(), "panel PNGs and/or data generation", r.logger)

	// Get the indexes of PNG panels that need to be included in the report
	pngPanels := selectPanels(dashboardData.Panels, r.conf.IncludePanelIDs, r.conf.ExcludePanelIDs, true)

	// Get the indexes of table panels that need to be included in the report
	tablePanels := selectPanels(dashboardData.Panels, r.conf.IncludePanelDataIDs, nil, false)

	errorCh := make(chan error, len(pngPanels)+len(tablePanels))

	wg := sync.WaitGroup{}

	for idx, panel := range dashboardData.Panels {
		if slices.Contains(pngPanels, idx) {
			wg.Add(1)

			r.pools[worker.Renderer].Do(func() {
				defer wg.Done()

				panelPNG, err := r.dashboard.PanelPNG(ctx, panel)
				if err != nil {
					errorCh <- fmt.Errorf("failed to fetch PNG data for panel %s: %w", panel.ID, err)
				}

				dashboardData.Panels[idx].EncodedImage = panelPNG
			})
		}

		if slices.Contains(tablePanels, idx) {
			wg.Add(1)

			r.pools[worker.Browser].Do(func() {
				defer wg.Done()

				panelData, err := r.dashboard.PanelCSV(ctx, panel)
				if err != nil {
					errorCh <- fmt.Errorf("failed to fetch CSV data for panel %s: %w", panel.ID, err)
				}

				dashboardData.Panels[idx].CSVData = panelData
			})
		}
	}

	wg.Wait()
	close(errorCh)

	errs := make([]error, 0, len(pngPanels)+len(tablePanels))

	for err := range errorCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to generate report: %w", errors.Join(errs...))
	}

	return nil
}

// generateHTMLFile generates HTML files for PDF.
func (r *Report) generateHTMLFile(dashboardData *dashboard.Data) (HTML, error) {
	var tmpl *template.Template

	var html HTML

	var err error

	// Template functions
	funcMap := template.FuncMap{
		// The name "inc" is what the function will be called in the template text.
		"inc": func(i int) int {
			return i + 1
		},

		"add": func(i float64) float64 {
			return i + 1
		},

		"mult": func(i int) int {
			return i*30 + 5
		},

		"embed": func(base64Content string) template.URL {
			for signature, mimeType := range popularSignatures {
				if strings.HasPrefix(base64Content, signature) {
					return template.URL(template.HTMLEscapeString(fmt.Sprintf("data:%s;base64,%s", mimeType, base64Content))) //nolint:gosec
				}
			}

			return template.URL(template.HTMLEscapeString(base64Content)) //nolint:gosec
		},

		"url": func(url string) template.URL {
			return template.URL(template.HTMLEscapeString(url)) //nolint:gosec
		},
	}

	// Make a new template for Body of the PDF
	if tmpl, err = template.New("report").Funcs(funcMap).ParseFS(templateFS, "templates/report.gohtml"); err != nil {
		return HTML{}, fmt.Errorf("error parsing PDF template: %w", err)
	}

	// Template data
	data := templateData{
		time.Now().Local().In(r.conf.Location).Format(r.conf.TimeFormat),
		dashboardData,
		r.conf,
	}

	// Render the template for Body of the PDF
	bufBody := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufBody, "report.gohtml", data); err != nil {
		return HTML{}, fmt.Errorf("error executing PDF template: %w", err)
	}

	html.Body = bufBody.String()

	// Make a new template for Header of the PDF
	if r.conf.HeaderTemplate != "" {
		tmpl, err = template.New("header").Funcs(funcMap).Parse(fmt.Sprintf(`{{define "header.gohtml"}}%s{{end}}`, r.conf.HeaderTemplate))
	} else {
		tmpl, err = template.New("header").Funcs(funcMap).ParseFS(templateFS, "templates/header.gohtml")
	}

	if err != nil {
		return HTML{}, fmt.Errorf("error parsing Header template: %w", err)
	}

	// Render the template for Header of the PDF
	bufHeader := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufHeader, "header.gohtml", data); err != nil {
		return HTML{}, fmt.Errorf("error executing Header template: %w", err)
	}

	html.Header = bufHeader.String()

	// Make a new template for Footer of the PDF
	if r.conf.FooterTemplate != "" {
		tmpl, err = template.New("footer").Funcs(funcMap).Parse(fmt.Sprintf(`{{define "footer.gohtml"}}%s{{end}}`, r.conf.FooterTemplate))
	} else {
		tmpl, err = template.New("footer").Funcs(funcMap).ParseFS(templateFS, "templates/footer.gohtml")
	}

	if err != nil {
		return HTML{}, fmt.Errorf("error parsing Footer template: %w", err)
	}

	// Render the template for Footer of the PDF
	bufFooter := &bytes.Buffer{}
	if err = tmpl.ExecuteTemplate(bufFooter, "footer.gohtml", data); err != nil {
		return HTML{}, fmt.Errorf("error executing Footer template: %w", err)
	}

	html.Footer = bufFooter.String()

	return html, nil
}

// renderPDF renders HTML page into PDF using Chromium.
func (r *Report) renderPDF(htmlReport HTML, writer io.Writer) error {
	defer helpers.TimeTrack(time.Now(), "pdf rendering", r.logger)

	// Create a new tab
	tab := r.chromeInstance.NewTab(r.logger, r.conf)
	defer tab.Close(r.logger)

	err := tab.PrintToPDF(chrome.PDFOptions{
		Header:      htmlReport.Header,
		Body:        htmlReport.Body,
		Footer:      htmlReport.Footer,
		Orientation: r.conf.Orientation,
	}, writer)
	if err != nil {
		return fmt.Errorf("error rendering PDF: %w", err)
	}

	return nil
}
