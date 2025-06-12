package report

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/worker"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	. "github.com/smartystreets/goconvey/convey"
)

var logger = log.NewNullLogger()

func TestReport(t *testing.T) {
	Convey("When generating a PDF", t, func() {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		workerPools := worker.Pools{
			worker.Browser:  worker.New(ctx, 6),
			worker.Renderer: worker.New(ctx, 2),
		}

		rep := New(
			logger,
			&config.Config{
				TimeFormat: time.UnixDate,
				Location:   time.Now().Location(),
			},
			nil,
			&chrome.LocalInstance{},
			workerPools,
			&dashboard.Dashboard{},
		)

		// Mock dashboard data
		dashData := dashboard.Data{
			Title: "My first dashboard",
			Panels: []dashboard.Panel{
				{ID: "1", EncodedImage: dashboard.PanelImage{Image: "iVBORw0KGgofsdfsdfsdf", MimeType: "image/png"}},
				{ID: "2", CSVData: [][]string{{"1", "2", "3"}, {"value1", "value2", "value3"}}},
			},
			Variables: "testvarvalue",
			TimeRange: dashboard.TimeRange{
				From: "1734194455000",
				To:   "1734194465000",
			},
		}

		Convey("When generating the HTML files", func() {
			html, err := rep.generateHTMLFile(&dashData)
			So(err, ShouldBeNil)

			Convey("The file should contain reference to the template data", func() {
				s := html.Body

				Convey("Including the Title", func() {
					So(html.Header, ShouldContainSubstring, "My first dashboard")
				})
				Convey("Including the variable values", func() {
					So(html.Header, ShouldContainSubstring, "testvarvalue")
				})
				Convey("and the images", func() {
					So(s, ShouldContainSubstring, "<table>")
					So(s, ShouldContainSubstring, "value1")
					So(s, ShouldContainSubstring, "<thead>")
				})
				Convey("and the table", func() {
					So(s, ShouldContainSubstring, "data:image/png")
					So(strings.Count(s, "data:image/png"), ShouldEqual, 1)

					So(s, ShouldContainSubstring, "image1")
				})
				Convey("and the time range", func() {
					// server time zone by shift hours timestamp
					// so just test for day and year
					So(html.Header, ShouldContainSubstring, "Sat Dec 14")
					So(html.Header, ShouldContainSubstring, "2024")
				})
			})
		})
	})
}
