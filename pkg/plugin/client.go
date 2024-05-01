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
	secrets          *Secrets
	queryParams      url.Values
	layout           string
	dashboardMode    string
}

var getPanelRetrySleepTime = time.Duration(10) * time.Second

// NewClient creates a new Grafana Client. Cookies and Authorization headers, if found,
// will be forwarded in the requests
// queryParams are Grafana template variable url values of the form var-{name}={value}, e.g. var-host=dev
func NewGrafanaClient(
	client *http.Client,
	grafanaAppURL string,
	secrets *Secrets,
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
		secrets,
		queryParams,
		layout,
		dashboardMode,
	}
}

// Forward auth header in the client request
// We fetch secrets from different sources
//  - From plugin config where users can configure a service account token either by
//    provisioning or set it iun UI
//  - If Grafana >= 10.3.0 is used and externalServiceAccounts is enabled, a token
//    will be generated for the plugin to use in API requests to Grafana. This token,
//    if found, will always have higher precendence to the one configured in the plugin
//  - Finally, if the request is coming from the browser, cookie will be retrieved.
//
// We should always prefer cookie to auth tokens as cookie will have correct permissions
// and scopes based on the user who is making the request. On the other hand, service
// account tokens, either configured from plugin or the one that is provisioned automatically
// for the plugin will always have broader scopes and permissions. 
func (g grafanaClient) forwardAuthHeader(r *http.Request) *http.Request {
	// If incoming request has cookies formward them
	// If cookie is not found, try Authorization header that is either configured
	// in config or fetched from externalServiceAccount
	if g.secrets.cookie != "" {
		r.Header.Set(backend.CookiesHeaderName, g.secrets.cookie)
	} else if g.secrets.token != "" {
		r.Header.Set(backend.OAuthIdentityTokenHeaderName, fmt.Sprintf("Bearer %s", g.secrets.token))
	}
	return r
}

// GetDashboard fetches dashboard from Grafana
func (g grafanaClient) GetDashboard(dashUID string) (Dashboard, error) {
	dashURL := g.getDashEndpoint(dashUID)

	// Create a new GET request
	req, err := http.NewRequest(http.MethodGet, dashURL, nil)
	if err != nil {
		return Dashboard{}, fmt.Errorf("error creating request for %s: %v", dashURL, err)
	}

	// Forward auth headers
	req = g.forwardAuthHeader(req)

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

	// Forward auth headers
	req = g.forwardAuthHeader(req)

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
