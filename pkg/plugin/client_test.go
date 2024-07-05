package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	. "github.com/smartystreets/goconvey/convey"
)

var testClient = http.Client{}

// We want our tests to run fast
func init() {
	getPanelRetrySleepTime = time.Duration(1) * time.Millisecond
}

func TestGrafanaClientFetchesDashboard(t *testing.T) {
	// Skip test if chrome is not available
	_, err := exec.LookPath("chrome")
	if err != nil {
		t.Skip("Chrome not found. Skipping test")
	}

	Convey("When fetching a Dashboard", t, func() {
		var requestURI []string
		requestCookie := ""
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestURI = append(requestURI, r.RequestURI)
			requestCookie = r.Header.Get(backend.CookiesHeaderName)
			w.Write([]byte(`{"dashboard": {"title": "foo"}}`))
		}))
		defer ts.Close()

		Convey("When using the Grafana client", func() {
			secrets := &Secrets{}
			secrets.cookieHeader = "cookie"
			config := &Config{
				AppURL:        ts.URL,
				Layout:        "simple",
				DashboardMode: "default",
			}
			grf := NewGrafanaClient(logger, &testClient, secrets, config, url.Values{})
			grf.Dashboard(context.Background(), "rYy7Paekz")

			Convey("It should use the v5 dashboards endpoint", func() {
				So(requestURI, ShouldContain, "/api/dashboards/uid/rYy7Paekz")
			})
			Convey("It should use cookie", func() {
				So(requestCookie, ShouldEqual, "cookie")
			})
		})
	})
}

func TestGrafanaClientFetchesPanelPNG(t *testing.T) {
	Convey("When fetching a panel PNG", t, func() {
		requestURI := ""
		requestHeaders := http.Header{}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestURI = r.RequestURI
			requestHeaders = r.Header
		}))
		defer ts.Close()

		secrets := &Secrets{}
		secrets.token = "token"
		config := &Config{
			AppURL:        ts.URL,
			Layout:        "simple",
			DashboardMode: "default",
		}
		variables := url.Values{}
		variables.Add("var-host", "servername")
		variables.Add("var-port", "adapter")

		cases := map[string]struct {
			client      GrafanaClient
			pngEndpoint string
		}{
			"client": {
				NewGrafanaClient(logger, &testClient, secrets, config, variables),
				"/render/d-solo/testDash/_",
			},
		}
		for _, cl := range cases {
			grf := cl.client
			grf.PanelPNG(
				Panel{44, "singlestat", "title", GridPos{0, 0, 0, 0}, ""},
				"testDash",
				TimeRange{"now-1h", "now"},
			)

			Convey("The client should use the render endpoint with the dashboard name", func() {
				So(requestURI, ShouldStartWith, cl.pngEndpoint)
			})

			Convey("The client should request the panel ID", func() {
				So(requestURI, ShouldContainSubstring, "panelId=44")
			})

			Convey("The client should request the time", func() {
				So(requestURI, ShouldContainSubstring, "from=now-1h")
				So(requestURI, ShouldContainSubstring, "to=now")
			})

			Convey("The client should insert auth token should in request header", func() {
				So(requestHeaders.Get("Authorization"), ShouldEqual, "Bearer token")
			})

			Convey("The client should pass variables in the request parameters", func() {
				So(requestURI, ShouldContainSubstring, "var-host=servername")
				So(requestURI, ShouldContainSubstring, "var-port=adapter")
			})

			Convey("The client should request singlestat panels at a smaller size", func() {
				So(requestURI, ShouldContainSubstring, "width=1000")
				So(requestURI, ShouldContainSubstring, "height=500")
			})
		}

		config.Layout = "grid"
		casesGridLayout := map[string]struct {
			client      GrafanaClient
			pngEndpoint string
		}{
			"client": {
				NewGrafanaClient(logger, &testClient, secrets, config, variables),
				"/render/d-solo/testDash/_",
			},
		}
		for _, cl := range casesGridLayout {
			grf := cl.client

			Convey("The client should request grid layout panels with width=2400 and height=216", func() {
				grf.PanelPNG(
					Panel{44, "graph", "title", GridPos{6, 24, 0, 0}, ""},
					"testDash",
					TimeRange{"now", "now-1h"},
				)
				So(requestURI, ShouldContainSubstring, "width=2400")
				So(requestURI, ShouldContainSubstring, "height=216")
			})
		}

	})
}

func TestGrafanaClientFetchPanelPNGErrorHandling(t *testing.T) {
	Convey("When trying to fetching a panel from the server sometimes returns an error", t, func() {
		try := 0

		// create a server that will return error on the first call
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if try < 1 {
				w.WriteHeader(http.StatusInternalServerError)
				try++
			}
		}))
		defer ts.Close()

		grf := NewGrafanaClient(logger, &testClient, &Secrets{}, &Config{AppURL: ts.URL}, url.Values{})

		_, err := grf.PanelPNG(
			Panel{44, "singlestat", "title", GridPos{0, 0, 0, 0}, ""},
			"testDash",
			TimeRange{"now-1h", "now"},
		)

		Convey("It should retry a couple of times if it receives errors", func() {
			So(err, ShouldBeNil)
		})
	})

	Convey("When trying to fetching a panel from the server consistently returns an error", t, func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		grf := NewGrafanaClient(logger, &testClient, &Secrets{}, &Config{AppURL: ts.URL}, url.Values{})

		_, err := grf.PanelPNG(
			Panel{44, "singlestat", "title", GridPos{0, 0, 0, 0}, ""},
			"testDash",
			TimeRange{"now-1h", "now"},
		)

		Convey("The Grafana API should return an error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}
