package client

import "errors"

var (
	ErrJavaScriptReturnedNoData = errors.New("javascript did not return any dashboard data")
	ErrDashboardHTTPError       = errors.New("dashboard request does not return 200 OK")
	ErrEmptyBlobURL             = errors.New("empty blob URL")
	ErrEmptyCSVData             = errors.New("empty csv data")
)
