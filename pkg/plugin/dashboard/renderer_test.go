package dashboard

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	. "github.com/smartystreets/goconvey/convey"
)

// We want our tests to run fast.
func init() {
	getPanelRetrySleepTime = time.Duration(1) * time.Millisecond
}

func TestFetchPanelPNG(t *testing.T) {
	Convey("When fetching a panel PNG", t, func() {
		requestURI := ""
		requestHeaders := http.Header{}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestURI = r.RequestURI
			requestHeaders = r.Header
		}))
		defer ts.Close()

		conf := config.Config{
			Layout:        "simple",
			DashboardMode: "default",
		}
		variables := url.Values{}
		variables.Add("var-host", "servername")
		variables.Add("var-port", "adapter")
		variables.Add("from", "now-1h")
		variables.Add("to", "now")

		dash, err := New(
			log.NewNullLogger(),
			&conf,
			http.DefaultClient,
			&chrome.LocalInstance{},
			ts.URL,
			"v11.1.0",
			&Model{Dashboard: struct {
				ID          int          `json:"id"`
				UID         string       `json:"uid"`
				Title       string       `json:"title"`
				Description string       `json:"description"`
				RowOrPanels []RowOrPanel `json:"panels"`
				Panels      []Panel
				Variables   url.Values
			}{
				UID:       "randomUID",
				Variables: variables,
			}},
			http.Header{
				backend.OAuthIdentityTokenHeaderName: []string{"Bearer token"},
			},
		)

		Convey("New dashboard should receive no errors", func() {
			So(err, ShouldBeNil)
		})

		_, err = dash.PanelPNG(t.Context(), Panel{ID: "44", Type: "singlestat", Title: "title", GridPos: GridPos{}})

		Convey("It should receives no errors", func() {
			So(err, ShouldBeNil)
		})

		Convey("The httpClient should use the render endpoint with the dashboard name", func() {
			So(requestURI, ShouldStartWith, "/render/d-solo/randomUID/_")
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

		// Use grid layout
		conf.Layout = "grid"

		dash, err = New(
			log.NewNullLogger(),
			&conf,
			http.DefaultClient,
			&chrome.LocalInstance{},
			ts.URL,
			"v11.1.0",
			&Model{Dashboard: struct {
				ID          int          `json:"id"`
				UID         string       `json:"uid"`
				Title       string       `json:"title"`
				Description string       `json:"description"`
				RowOrPanels []RowOrPanel `json:"panels"`
				Panels      []Panel
				Variables   url.Values
			}{
				UID:       "randomUID",
				Variables: variables,
			}},
			http.Header{
				backend.OAuthIdentityTokenHeaderName: []string{"token"},
			},
		)

		Convey("New dashboard should receive no errors using grid layout", func() {
			So(err, ShouldBeNil)
		})

		_, err = dash.PanelPNG(t.Context(), Panel{ID: "44", Type: "graph", Title: "title", GridPos: GridPos{H: 6, W: 24}})

		Convey("It should receives no errors using grid layout", func() {
			So(err, ShouldBeNil)
		})

		Convey("The httpClient should request singlestat panels at grid layout size", func() {
			So(requestURI, ShouldContainSubstring, "width=1536")
			So(requestURI, ShouldContainSubstring, "height=216")
		})
	})
}
