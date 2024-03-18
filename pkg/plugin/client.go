package plugin

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Client is a Grafana API client
type GrafanaClient interface {
	GetDashboard(dashUID string) (Dashboard, error)
	GetPanelPNG(p Panel, dashUID string, t TimeRange) (io.ReadCloser, error)
}

// grafanaClient is the struct that will implement required interfaces
type grafanaClient struct {
	client           *http.Client
	url              string
	getDashEndpoint  func(dashUID string) string
	getPanelEndpoint func(dashUID string, vals url.Values) string
	cookies          string
	queryParams      url.Values
	layout           string
	dashboardMode    string
}

var getPanelRetrySleepTime = time.Duration(10) * time.Second

// NewClient creates a new Grafana Client. If cookies is the non-empty string,
// cookie will be forwarded in the requests.
// queryParams are Grafana template variable url values of the form var-{name}={value}, e.g. var-host=dev
func NewGrafanaClient(
	client *http.Client,
	grafanaAppURL string,
	cookie string,
	queryParams url.Values,
	layout string,
	dashboardMode string,
) GrafanaClient {
	// Get dashboard URL
	getDashEndpoint := func(dashUID string) string {
		dashURL := fmt.Sprintf("%s/api/dashboards/uid/%s", grafanaAppURL, dashUID)
		if len(queryParams) > 0 {
			dashURL = fmt.Sprintf("%s?%s", dashURL, queryParams.Encode())
		}
		return dashURL
	}

	// Get Panel URL
	getPanelEndpoint := func(dashUID string, vals url.Values) string {
		return fmt.Sprintf("%s/render/d-solo/%s/_?%s", grafanaAppURL, dashUID, vals.Encode())
	}
	return grafanaClient{
		client,
		grafanaAppURL,
		getDashEndpoint,
		getPanelEndpoint,
		cookie,
		queryParams,
		layout,
		dashboardMode,
	}
}

func (g grafanaClient) GetDashboard(dashUID string) (Dashboard, error) {
	dashURL := g.getDashEndpoint(dashUID)

	// Create a new GET request
	req, err := http.NewRequest(http.MethodGet, dashURL, nil)
	if err != nil {
		return Dashboard{}, fmt.Errorf("error creating request for %s: %v", dashURL, err)
	}

	// If incoming request has cookies formward them
	if g.cookies != "" {
		req.Header.Set(backend.CookiesHeaderName, g.cookies)
	}

	// Make request
	resp, err := g.client.Do(req)
	if err != nil {
		return Dashboard{}, fmt.Errorf("error executing request for %s: %v", dashURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Dashboard{}, fmt.Errorf("error reading response body from %s: %v", dashURL, err)
	}

	if resp.StatusCode != 200 {
		return Dashboard{}, fmt.Errorf(
			"error obtaining dashboard from %s. Got Status %v, message: %v ",
			dashURL,
			resp.Status,
			string(body),
		)
	}
	return NewDashboard(body, g.queryParams, g.dashboardMode), nil
}

func (g grafanaClient) GetPanelPNG(p Panel, dashUID string, t TimeRange) (io.ReadCloser, error) {
	panelURL := g.getPanelURL(p, dashUID, t)

	// Create a new request for panel
	req, err := http.NewRequest(http.MethodGet, panelURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for %s: %v", panelURL, err)
	}

	// Forward cookies from incoming request
	if g.cookies != "" {
		req.Header.Set(backend.CookiesHeaderName, g.cookies)
	}

	// Make request
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request for %s: %v", panelURL, err)
	}

	// Do multiple tries to get panel before giving up
	for retries := 1; retries < 3 && resp.StatusCode != 200; retries++ {
		delay := getPanelRetrySleepTime * time.Duration(retries)
		time.Sleep(delay)
		resp, err = g.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error executing retry request for %s: %v", panelURL, err)
		}
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error rendering panel: %s", resp.Status)
	}
	return resp.Body, nil
}

func (g grafanaClient) getPanelURL(p Panel, dashUID string, t TimeRange) string {
	values := url.Values{}
	values.Add("theme", "light")
	values.Add("panelId", strconv.Itoa(p.Id))
	values.Add("from", t.From)
	values.Add("to", t.To)

	// If using a grid layout we use 100px for width and 36px for height scaling.
	// Grafana panels are fitted into 24 units width and height units are said to
	// 30px in docs but 36px seems to be better.
	//
	// In simple layout we create panels with 1000x500 resolution always and include
	// them one in each page of report
	if g.layout == "grid" {
		width := int(p.GridPos.W * 100)
		height := int(p.GridPos.H * 36)
		values.Add("width", strconv.Itoa(width))
		values.Add("height", strconv.Itoa(height))
	} else {
		values.Add("width", "1000")
		values.Add("height", "500")
	}

	// Add templated queryParams to URL
	for k, v := range g.queryParams {
		for _, singleValue := range v {
			values.Add(k, singleValue)
		}
	}
	return g.getPanelEndpoint(dashUID, values)
}
