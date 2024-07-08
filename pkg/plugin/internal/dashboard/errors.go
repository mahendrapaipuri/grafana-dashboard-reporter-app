package dashboard

import "errors"

var (
	ErrNoPanels        = errors.New("no panels found in browser data")
	ErrNoDashboardData = errors.New("no dashboard data found in browser data")
)
