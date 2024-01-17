package plugin

import (
	"errors"
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

type grafanaClient struct {
	client           *http.Client
	url              string
	getDashEndpoint  func(dashUID string) string
	getPanelEndpoint func(dashUID string, vals url.Values) string
	cookies          string
	variables        url.Values
	useGridLayout    bool
}

var getPanelRetrySleepTime = time.Duration(10) * time.Second

// NewClient creates a new Grafana Client. If apiToken is the empty string,
// authorization headers will be omitted from requests.
// variables are Grafana template variable url values of the form var-{name}={value}, e.g. var-host=dev
func NewGrafanaClient(client *http.Client, grafanaAppURL string, cookie string, variables url.Values, useGridLayout bool) GrafanaClient {
	// Get dashboard URL
	getDashEndpoint := func(dashUID string) string {
		dashURL := grafanaAppURL + "/api/dashboards/uid/" + dashUID
		if len(variables) > 0 {
			dashURL = dashURL + "?" + variables.Encode()
		}
		return dashURL
	}

	// Get Panel URL
	getPanelEndpoint := func(dashUID string, vals url.Values) string {
		return fmt.Sprintf("%s/render/d-solo/%s/_?%s", grafanaAppURL, dashUID, vals.Encode())
	}
	return grafanaClient{client, grafanaAppURL, getDashEndpoint, getPanelEndpoint, cookie, variables, useGridLayout}
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
		return Dashboard{}, fmt.Errorf("error obtaining dashboard from %s. Got Status %v, message: %v ", dashURL, resp.Status, string(body))
	}
	return NewDashboard(body, g.variables), nil
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
		_, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		return nil, errors.New("error panel render: " + resp.Status)
	}
	return resp.Body, nil
}

func (g grafanaClient) getPanelURL(p Panel, dashUID string, t TimeRange) string {
	values := url.Values{}
	values.Add("theme", "light")
	values.Add("panelId", strconv.Itoa(p.Id))
	values.Add("from", t.From)
	values.Add("to", t.To)

	if g.useGridLayout {
		width := int(p.GridPos.W * 40)
		height := int(p.GridPos.H * 40)
		values.Add("width", strconv.Itoa(width))
		values.Add("height", strconv.Itoa(height))
	} else {
		if p.Is(SingleStat) {
			values.Add("width", "300")
			values.Add("height", "150")
		} else if p.Is(Text) {
			values.Add("width", "1000")
			values.Add("height", "100")
		} else {
			values.Add("width", "1000")
			values.Add("height", "500")
		}
	}

	for k, v := range g.variables {
		for _, singleValue := range v {
			values.Add(k, singleValue)
		}
	}

	url := g.getPanelEndpoint(dashUID, values)
	return url
}
