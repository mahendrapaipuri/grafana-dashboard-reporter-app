package plugin

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"text/template"

	"github.com/google/uuid"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Report groups functions related to genrating the report.
// After reading and closing the pdf returned by Generate(), call Clean() to delete the pdf file as well the temporary build files
type Report interface {
	Generate() (io.ReadCloser, error)
	Title() string
	Clean()
}

// Data structures used inside TeX template
type templateData struct {
	Dashboard
	TimeRange
	GrafanaClient
}

// Report config
type ReportConfig struct {
	dashTitle        string
	dashUID          string
	timeRange        TimeRange
	texTemplate      string
	stagingDir       string
	maxRenderWorkers int
	useGridLayout    bool
}

// report struct
type report struct {
	logger log.Logger
	client GrafanaClient
	cfg    *ReportConfig
}

const (
	imgDir        = "images"
	reportTeXFile = "report.tex"
	reportPDF     = "report.pdf"
)

func newReport(logger log.Logger, client GrafanaClient, config *ReportConfig) *report {
	if config.texTemplate == "" {
		if config.useGridLayout {
			config.texTemplate = defaultGridTemplate
		} else {
			config.texTemplate = defaultTemplate
		}
	}
	config.stagingDir = filepath.Join(config.stagingDir, uuid.New().String())
	return &report{logger, client, config}
}

// New creates a new Report.
// texTemplate is the content of a LaTex template file. If empty, a default tex template is used.
func NewReport(logger log.Logger, client GrafanaClient, config *ReportConfig) Report {
	return newReport(logger, client, config)
}

// Generate returns the report.pdf file.  After reading this file it should be Closed()
// After closing the file, call report.Clean() to delete the file as well the temporary build files
func (r *report) Generate() (io.ReadCloser, error) {
	// Get dashboard JSON model
	dash, err := r.client.GetDashboard(r.cfg.dashUID)
	if err != nil {
		return nil, fmt.Errorf("error fetching dashboard %s: %v", r.cfg.dashUID, err)
	}
	r.cfg.dashTitle = dash.Title

	// Render panel PNGs in parallel using max workers configured in plugin
	if err = r.renderPNGsParallel(dash); err != nil {
		return nil, fmt.Errorf("error rendering PNGs in parallel for dash %s: %v", dash.Title, err)
	}

	// Generate TeX file with fetched panel PNGs
	if err = r.generateTeXFile(dash); err != nil {
		return nil, fmt.Errorf("error generating TeX file for dash %s: %v", dash.Title, err)
	}

	// Compile TeX into PDF
	return r.runLaTeX()
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
	err := os.RemoveAll(r.cfg.stagingDir)
	if err != nil {
		r.logger.Warn("error cleaning up staging dir", "err", err, "dashTitle", r.cfg.dashTitle)
	}
}

// Get path to images directory
func (r *report) imgDirPath() string {
	return filepath.Join(r.cfg.stagingDir, imgDir)
}

// Get path to PDF
func (r *report) pdfPath() string {
	return filepath.Join(r.cfg.stagingDir, reportPDF)
}

// Get path to TeX file
func (r *report) texPath() string {
	return filepath.Join(r.cfg.stagingDir, reportTeXFile)
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
	var file *os.File
	var err error

	// Get panel
	if body, err = r.client.GetPanelPNG(p, r.cfg.dashUID, r.cfg.timeRange); err != nil {
		return fmt.Errorf("error getting panel %s: %v", p.Title, err)
	}
	defer body.Close()

	// Create directory to store PNG files and get file handler
	if err = os.MkdirAll(r.imgDirPath(), 0750); err != nil {
		return fmt.Errorf("error creating img directory: %v", err)
	}
	imgFileName := fmt.Sprintf("image%d.png", p.Id)
	if file, err = os.Create(filepath.Join(r.imgDirPath(), imgFileName)); err != nil {
		return fmt.Errorf("error creating image file: %v", err)
	}
	defer file.Close()

	// Copy PNG to file
	if _, err = io.Copy(file, body); err != nil {
		return fmt.Errorf("error copying body to file: %v", err)
	}
	return nil
}

// Generate TeX file for dashboard
func (r *report) generateTeXFile(dash Dashboard) error {
	var file *os.File
	var tmpl *template.Template
	var err error

	// Make directory and file handle for TeX file
	if err := os.MkdirAll(r.cfg.stagingDir, 0750); err != nil {
		return fmt.Errorf("error creating temporary directory at %s: %v", r.cfg.stagingDir, err)
	}
	if file, err = os.Create(r.texPath()); err != nil {
		return fmt.Errorf("error creating tex file at %v : %v", r.texPath(), err)
	}
	defer file.Close()

	// Make a new template
	if tmpl, err = template.New("report").Delims("[[", "]]").Parse(r.cfg.texTemplate); err != nil {
		return fmt.Errorf("error parsing template '%s': %v", r.cfg.texTemplate, err)
	}

	// Render the template
	if err = tmpl.Execute(file, templateData{dash, r.cfg.timeRange, r.client}); err != nil {
		return fmt.Errorf("error executing tex template: %v", err)
	}
	return nil
}

// Compile TeX into PDF
func (r *report) runLaTeX() (io.ReadCloser, error) {
	// Execute pdflatex preprocessing
	cmdPre := exec.Command("pdflatex", "-halt-on-error", "-draftmode", reportTeXFile)
	cmdPre.Dir = r.cfg.stagingDir
	if outBytesPre, errPre := cmdPre.CombinedOutput(); errPre != nil {
		return nil, fmt.Errorf("error calling LaTeX preprocessing: %v. Latex preprocessing failed with output: %s ", errPre, string(outBytesPre))
	}
	cmd := exec.Command("pdflatex", "-halt-on-error", reportTeXFile)
	cmd.Dir = r.cfg.stagingDir
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("error calling LaTeX: %q. Latex failed with output: %s ", err, string(outBytes))
	}
	return os.Open(r.pdfPath())
}
