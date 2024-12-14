package chrome

import (
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
)

// PDFOptions contains the templated HTML Body, Header and Footer strings.
type PDFOptions struct {
	Header string
	Body   string
	Footer string

	Orientation string
}

// Instance is the interface remote and local chrome must implement.
type Instance interface {
	NewTab(logger log.Logger, conf *config.Config) *Tab
	Name() string
	Close(logger log.Logger)
}
