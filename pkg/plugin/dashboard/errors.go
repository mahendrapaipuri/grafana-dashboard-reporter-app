package dashboard

import "errors"

var (
	ErrNoPanels                 = errors.New("no panels found in browser data")
	ErrNoDashboardData          = errors.New("no dashboard data found")
	ErrJavaScriptReturnedNoData = errors.New("javascript did not return any dashboard data")
	ErrDashboardHTTPError       = errors.New("dashboard request does not return 200 OK")
	ErrEmptyBlobURL             = errors.New("empty blob URL")
	ErrEmptyPanelElement        = errors.New("no data element found in the panel")
	ErrEmptyCSVData             = errors.New("empty csv data")
)
