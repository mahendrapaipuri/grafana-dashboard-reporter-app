package dashboard

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strconv"
	"time"
)

var getPanelRetrySleepTime = time.Duration(10) * time.Second

// PanelPNG returns encoded PNG image of a given panel.
func (d *Dashboard) PanelPNG(ctx context.Context, p Panel) (PanelImage, error) {
	// Get panel render URL
	panelURL := d.panelPNGURL(p, d.model.Dashboard.UID)

	// Create a new request for panel
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, panelURL, nil)
	if err != nil {
		return PanelImage{}, fmt.Errorf("error creating request for %s: %w", panelURL, err)
	}

	// Forward auth headers
	for name, values := range d.authHeader {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Make request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return PanelImage{}, fmt.Errorf("error executing request for %s: %w", panelURL, err)
	}
	defer resp.Body.Close()

	// Do multiple tries to get panel before giving up
	for retries := 1; retries < 3 && resp.StatusCode != http.StatusOK; retries++ {
		resp.Body.Close()

		delay := getPanelRetrySleepTime * time.Duration(retries)
		time.Sleep(delay)

		resp, err = d.httpClient.Do(req)
		if err != nil {
			return PanelImage{}, fmt.Errorf("error executing retry request for %s: %w", panelURL, err)
		}
		defer resp.Body.Close()
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PanelImage{}, fmt.Errorf("error reading response body of panel PNG: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return PanelImage{}, fmt.Errorf(
			"%w: URL: %s. Status: %s, message: %s",
			ErrDashboardHTTPError,
			panelURL,
			resp.Status,
			string(body),
		)
	}

	sb := &bytes.Buffer{}
	sb.Grow(base64.StdEncoding.EncodedLen(int(resp.ContentLength)))

	encoder := base64.NewEncoder(base64.StdEncoding, sb)

	if _, err = encoder.Write(body); err != nil {
		return PanelImage{}, fmt.Errorf("error reading response body of panel PNG: %w", err)
	}

	return PanelImage{
		Image:    sb.String(),
		MimeType: "image/png",
	}, nil
}

// panelPNGURL returns the URL to fetch panel PNd.
func (d *Dashboard) panelPNGURL(p Panel, dashUID string) string {
	values := maps.Clone(d.model.Dashboard.Variables)
	values.Add("theme", d.conf.Theme)
	values.Add("panelId", p.ID)

	if d.conf.TimeZone != "" {
		values.Add("timezone", d.conf.TimeZone)
	}

	// If using a grid layout we use 100px for width and 36px for height scalind.
	// Grafana panels are fitted into 24 units width and height units are said to
	// 30px in docs but 36px seems to be better.
	//
	// In simple layout we create panels with 1000x500 resolution always and include
	// them one in each page of report
	if d.conf.Layout == "grid" {
		width := int(p.GridPos.W * 100)
		height := int(p.GridPos.H * 36)

		values.Add("width", strconv.Itoa(width))
		values.Add("height", strconv.Itoa(height))
	} else {
		values.Add("width", "1000")
		values.Add("height", "500")
	}

	// Get Panel API endpoint
	return fmt.Sprintf("%s/render/d-solo/%s/_?%s", d.appURL, dashUID, values.Encode())
}
