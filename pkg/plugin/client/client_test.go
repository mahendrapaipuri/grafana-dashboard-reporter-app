package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/worker"
	. "github.com/smartystreets/goconvey/convey"
)

// We want our tests to run fast
func init() {
	getPanelRetrySleepTime = time.Duration(1) * time.Millisecond
}

func TestGrafanaClientFetchesDashboardWithLocalChrome(t *testing.T) {
	var execPath string
	locations := []string{
		// Mac
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		// Windows
		"chrome.exe",
		// Linux
		"google-chrome",
		"chrome",
	}

	for _, path := range locations {
		found, err := exec.LookPath(path)
		if err == nil {
			execPath = found

			break
		}
	}

	// Skip test if chrome is not available
	if execPath == "" {
		t.Skip("Chrome not found. Skipping test")
	}

	Convey("When fetching a Dashboard", t, func() {
		chromeInstance, err := chrome.NewLocalBrowserInstance(context.Background(), log.NewNullLogger(), true)
		Convey("setup a chrome browser should not error", func() {
			So(err, ShouldBeNil)
		})

		var requestURI []string
		requestCookie := ""
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestURI = append(requestURI, r.RequestURI)
			requestCookie = r.Header.Get(backend.CookiesHeaderName)

			if _, err := w.Write([]byte(`{"dashboard": {"title": "foo","panels":[{"type":"singlestat", "id":0}]}}`)); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}))
		defer ts.Close()

		Convey("When using the Grafana httpClient", func() {
			credential := Credential{HeaderName: backend.CookiesHeaderName, HeaderValue: "cookie"}
			conf := config.Config{
				Layout:        "simple",
				DashboardMode: "default",
			}

			ctx := context.Background()
			workerPools := worker.Pools{
				worker.Browser:  worker.New(ctx, 6),
				worker.Renderer: worker.New(ctx, 2),
			}
			grf := New(log.NewNullLogger(), conf, http.DefaultClient, chromeInstance, workerPools, ts.URL, credential, url.Values{})
			_, err := grf.Dashboard(context.Background(), "randomDashUID")

			Convey("It should receive no errors", func() {
				So(errors.Is(err, dashboard.ErrNoDashboardData), ShouldBeTrue)
			})

			Convey("It should use the v5 dashboards endpoint", func() {
				So(requestURI, ShouldContain, "/api/dashboards/uid/randomDashUID")
			})
			Convey("It should use cookie", func() {
				So(requestCookie, ShouldEqual, "cookie")
			})
		})
	})
}

func TestGrafanaClientFetchesDashboardWithRemoteChrome(t *testing.T) {
	// Skip test if chrome is not available
	chromeRemoteAddr, ok := os.LookupEnv("CHROME_REMOTE_URL")
	if !ok {
		t.Skip("CHROME_REMOTE_URL unset. Skipping test")
	}

	Convey("When fetching a Dashboard", t, func() {
		chromeInstance, err := chrome.NewRemoteBrowserInstance(context.Background(), log.NewNullLogger(), chromeRemoteAddr)
		Convey("setup a chrome browser should not error", func() {
			So(err, ShouldBeNil)
		})

		var requestURI []string
		requestCookie := ""
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestURI = append(requestURI, r.RequestURI)
			requestCookie = r.Header.Get(backend.CookiesHeaderName)

			if _, err := w.Write([]byte(`{"dashboard": {"title": "foo","panels":[{"type":"singlestat", "id":0}]}}`)); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}))
		defer ts.Close()

		Convey("When using the Grafana httpClient", func() {
			credential := Credential{HeaderName: backend.CookiesHeaderName, HeaderValue: "cookie"}
			conf := config.Config{
				Layout:        "simple",
				DashboardMode: "default",
			}

			ctx := context.Background()
			workerPools := worker.Pools{
				worker.Browser:  worker.New(ctx, 6),
				worker.Renderer: worker.New(ctx, 2),
			}
			grf := New(log.NewNullLogger(), conf, http.DefaultClient, chromeInstance, workerPools, ts.URL, credential, url.Values{})
			_, err := grf.Dashboard(context.Background(), "randomDashUID")

			Convey("It should receive no errors", func() {
				So(errors.Is(err, dashboard.ErrNoDashboardData), ShouldBeTrue)
			})

			Convey("It should use the v5 dashboards endpoint", func() {
				So(requestURI, ShouldContain, "/api/dashboards/uid/randomDashUID")
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

		credential := Credential{HeaderName: backend.OAuthIdentityTokenHeaderName, HeaderValue: "Bearer token"}
		conf := config.Config{
			Layout:        "simple",
			DashboardMode: "default",
		}
		variables := url.Values{}
		variables.Add("var-host", "servername")
		variables.Add("var-port", "adapter")

		ctx := context.Background()
		workerPools := worker.Pools{
			worker.Browser:  worker.New(ctx, 6),
			worker.Renderer: worker.New(ctx, 2),
		}
		cases := map[string]struct {
			client      Grafana
			pngEndpoint string
		}{
			"httpClient": {
				New(log.NewNullLogger(), conf, http.DefaultClient, nil, workerPools, ts.URL, credential, variables),
				"/render/d-solo/testDash/_",
			},
		}
		for _, cl := range cases {
			grf := cl.client
			_, err := grf.PanelPNG(
				context.Background(), "testDash",
				dashboard.Panel{ID: 44, Type: "singlestat", Title: "title", GridPos: dashboard.GridPos{}},
				dashboard.TimeRange{From: "now-1h", To: "now"},
			)

			Convey("It should receives no errors", func() {
				So(err, ShouldBeNil)
			})

			Convey("The httpClient should use the render endpoint with the dashboard name", func() {
				So(requestURI, ShouldStartWith, cl.pngEndpoint)
			})

			Convey("The httpClient should request the panel ID", func() {
				So(requestURI, ShouldContainSubstring, "panelId=44")
			})

			Convey("The httpClient should request the time", func() {
				So(requestURI, ShouldContainSubstring, "from=now-1h")
				So(requestURI, ShouldContainSubstring, "to=now")
			})

			Convey("The httpClient should insert auth token should in request header", func() {
				So(requestHeaders.Get("Authorization"), ShouldEqual, "Bearer token")
			})

			Convey("The httpClient should pass variables in the request parameters", func() {
				So(requestURI, ShouldContainSubstring, "var-host=servername")
				So(requestURI, ShouldContainSubstring, "var-port=adapter")
			})

			Convey("The httpClient should request singlestat panels at a smaller size", func() {
				So(requestURI, ShouldContainSubstring, "width=1000")
				So(requestURI, ShouldContainSubstring, "height=500")
			})
		}
		conf.Layout = "grid"
		casesGridLayout := map[string]struct {
			client      Grafana
			pngEndpoint string
		}{
			"httpClient": {
				New(log.NewNullLogger(), conf, http.DefaultClient, nil, workerPools, ts.URL, credential, variables),
				"/render/d-solo/testDash/_",
			},
		}
		for _, cl := range casesGridLayout {
			grf := cl.client

			Convey("The httpClient should request grid layout panels with width=2400 and height=216", func() {
				_, err := grf.PanelPNG(context.Background(), "testDash",
					dashboard.Panel{ID: 44, Type: "graph", Title: "title", GridPos: dashboard.GridPos{H: 6, W: 24}},
					dashboard.TimeRange{From: "now", To: "now-1h"},
				)

				So(err, ShouldBeNil)
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

		ctx := context.Background()
		workerPools := worker.Pools{
			worker.Browser:  worker.New(ctx, 6),
			worker.Renderer: worker.New(ctx, 2),
		}
		grf := New(log.NewNullLogger(), config.Config{}, http.DefaultClient, nil, workerPools, ts.URL, Credential{}, url.Values{})

		_, err := grf.PanelPNG(context.Background(), "testDash",
			dashboard.Panel{ID: 44, Type: "singlestat", Title: "title", GridPos: dashboard.GridPos{}},
			dashboard.TimeRange{From: "now-1h", To: "now"},
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

		ctx := context.Background()
		workerPools := worker.Pools{
			worker.Browser:  worker.New(ctx, 6),
			worker.Renderer: worker.New(ctx, 2),
		}
		grf := New(log.NewNullLogger(), config.Config{}, http.DefaultClient, nil, workerPools, ts.URL, Credential{}, url.Values{})

		_, err := grf.PanelPNG(context.Background(), "testDash",
			dashboard.Panel{ID: 44, Type: "singlestat", Title: "title", GridPos: dashboard.GridPos{}},
			dashboard.TimeRange{From: "now-1h", To: "now"},
		)

		Convey("The Grafana API should return an error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}
