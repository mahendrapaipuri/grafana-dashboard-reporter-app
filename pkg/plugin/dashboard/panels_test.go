package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/worker"
	. "github.com/smartystreets/goconvey/convey"
)

var muLock sync.RWMutex

func TestDashboardFetchWithLocalChrome(t *testing.T) {
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
		// Get CWD
		cwd, err := os.Getwd()

		Convey("getting CWD should not error", func() {
			So(err, ShouldBeNil)
		})

		// Read sample HTML file
		data, err := os.ReadFile(filepath.Join(cwd, "testdata/dashboard.html"))

		Convey("setup a dashboard HTML page should not error", func() {
			So(err, ShouldBeNil)
		})

		chromeInstance, err := chrome.NewLocalBrowserInstance(context.Background(), log.NewNullLogger(), true)
		defer chromeInstance.Close(log.NewNullLogger()) //nolint:staticcheck

		Convey("setup a chrome browser should not error", func() {
			So(err, ShouldBeNil)
		})

		var requestURI []string

		requestCookie := ""

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			muLock.Lock()
			requestURI = append(requestURI, r.RequestURI)
			requestCookie = r.Header.Get(backend.CookiesHeaderName)
			muLock.Unlock()

			if _, err := w.Write(data); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}
		}))
		defer ts.Close()

		Convey("When using the panels fetcher", func() {
			conf := config.Config{
				Layout:            "simple",
				DashboardMode:     "default",
				HTTPClientOptions: httpclient.Options{Timeouts: &httpclient.DefaultTimeoutOptions},
			}

			ctx := context.Background()
			workerPools := worker.Pools{
				worker.Browser:  worker.New(ctx, 6),
				worker.Renderer: worker.New(ctx, 2),
			}
			dash := New(
				log.NewNullLogger(),
				&conf,
				http.DefaultClient,
				chromeInstance,
				workerPools,
				ts.URL,
				"v11.4.0",
				&Model{Dashboard: struct {
					ID          int          `json:"id"`
					UID         string       `json:"uid"`
					Title       string       `json:"title"`
					Description string       `json:"description"`
					RowOrPanels []RowOrPanel `json:"panels"`
					Panels      []Panel
					Variables   url.Values
				}{
					UID: "randomUID",
				}},
				http.Header{
					backend.CookiesHeaderName: []string{"cookie"},
				},
			)
			d, err := dash.panelData(context.Background())

			Convey("It should receive no errors", func() {
				So(err, ShouldBeNil)
			})
			Convey("It should use dashboards endpoint", func() {
				So(requestURI, ShouldContain, "/d/randomUID/_?")
			})
			Convey("It should use cookie", func() {
				So(requestCookie, ShouldEqual, "cookie")
			})
			Convey("It should return dashboard data", func() {
				So(d, ShouldHaveLength, 4)
			})
		})
	})
}

func TestDashboardFetchWithRemoteChrome(t *testing.T) {
	// Skip test if chrome is not available
	chromeRemoteAddr, ok := os.LookupEnv("CHROME_REMOTE_URL")
	if !ok {
		t.Skip("CHROME_REMOTE_URL unset. Skipping test")
	}

	Convey("When fetching a Dashboard", t, func() {
		// Get CWD
		cwd, err := os.Getwd()

		Convey("getting CWD should not error", func() {
			So(err, ShouldBeNil)
		})

		// Read sample HTML file
		data, err := os.ReadFile(filepath.Join(cwd, "testdata/dashboard.html"))

		Convey("setup a dashboard HTML page should not error", func() {
			So(err, ShouldBeNil)
		})

		chromeInstance, err := chrome.NewRemoteBrowserInstance(
			context.Background(),
			log.NewNullLogger(),
			chromeRemoteAddr,
		)

		Convey("setup a chrome browser should not error", func() {
			So(err, ShouldBeNil)
		})

		var requestURI []string

		requestCookie := ""

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			muLock.Lock()
			requestURI = append(requestURI, r.RequestURI)
			requestCookie = r.Header.Get(backend.CookiesHeaderName)
			muLock.Unlock()

			if _, err := w.Write(data); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}
		}))
		defer ts.Close()

		Convey("When using the Grafana httpClient", func() {
			conf := config.Config{
				Layout:            "simple",
				DashboardMode:     "default",
				HTTPClientOptions: httpclient.Options{Timeouts: &httpclient.DefaultTimeoutOptions},
			}

			ctx := context.Background()
			workerPools := worker.Pools{
				worker.Browser:  worker.New(ctx, 6),
				worker.Renderer: worker.New(ctx, 2),
			}
			dash := New(
				log.NewNullLogger(),
				&conf,
				http.DefaultClient,
				chromeInstance,
				workerPools,
				ts.URL,
				"v11.4.0",
				&Model{Dashboard: struct {
					ID          int          `json:"id"`
					UID         string       `json:"uid"`
					Title       string       `json:"title"`
					Description string       `json:"description"`
					RowOrPanels []RowOrPanel `json:"panels"`
					Panels      []Panel
					Variables   url.Values
				}{
					UID: "randomUID",
				}},
				http.Header{
					backend.CookiesHeaderName: []string{"cookie"},
				},
			)
			d, err := dash.panelData(context.Background())

			Convey("It should receive no errors", func() {
				So(err, ShouldBeNil)
			})
			Convey("It should use the v5 dashboards endpoint", func() {
				So(requestURI, ShouldContain, "/d/randomUID/_?")
			})
			Convey("It should use cookie", func() {
				So(requestCookie, ShouldEqual, "cookie")
			})
			Convey("It should return dashboard data", func() {
				So(d, ShouldHaveLength, 4)
			})
		})
	})
}

func TestDashboardCreatePanels(t *testing.T) {
	Convey("When creating panels for Dashboard", t, func() {
		dash := New(
			log.NewNullLogger(),
			nil,
			nil,
			nil,
			worker.Pools{},
			"http://localhost:3000",
			"v11.4.0",
			&Model{Dashboard: struct {
				ID          int          `json:"id"`
				UID         string       `json:"uid"`
				Title       string       `json:"title"`
				Description string       `json:"description"`
				RowOrPanels []RowOrPanel `json:"panels"`
				Panels      []Panel
				Variables   url.Values
			}{
				UID: "randomUID",
			}},
			nil,
		)

		dashDataString := `[{"width":940,"height":258,"x":0,"y":0,"id":"12"},{"width":940,"height":258,"x":940,"y":0,"id":"26"},{"width":940,"height":258,"x":0,"y":0,"id":"27"}]`

		var dashData []interface{}
		err := json.Unmarshal([]byte(dashDataString), &dashData)

		Convey("setup dashboard data unmarshal", func() {
			So(err, ShouldBeNil)
		})

		panels, err := dash.createPanels(dashData)

		Convey("It should receive no errors", func() {
			So(err, ShouldBeNil)
		})
		Convey("It should all panels from dashboard browser data", func() {
			So(panels, ShouldHaveLength, 3)
			So(panels[0].ID, ShouldEqual, "12")
			So(panels[1].ID, ShouldEqual, "26")
			So(panels[2].ID, ShouldEqual, "27")
		})
	})
}
