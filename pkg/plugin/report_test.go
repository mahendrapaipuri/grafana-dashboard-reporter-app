package plugin

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	. "github.com/smartystreets/goconvey/convey"
)

const dashJSON = `
{"dashboard":
	{
		"title":"My first dashboard",
		"panels": 
			[{"type":"singlestat", "id":1},
			 {"type":"graph", "id":22},
			 {"type":"singlestat", "id":33},
			 {"type":"graph", "id":44},
			 {"type":"graph", "id":55},
			 {"type":"graph", "id":66},
			 {"type":"graph", "id":77},
			 {"type":"graph", "id":88},
			 {"type":"graph", "id":99}]
	},
"meta":
	{"Slug":"testDash"}
}`

var logger = log.DefaultLogger

type mockGrafanaClient struct {
	getPanelCallCount int
	variables         url.Values
}

func (m *mockGrafanaClient) Dashboard(_ context.Context, dashName string) (Dashboard, error) {
	return NewDashboard([]byte(dashJSON), nil, m.variables, &Config{})
}

func (m *mockGrafanaClient) PanelPNG(p Panel, dashName string, t TimeRange) (string, error) {
	m.getPanelCallCount++
	return "iVBORw0KGgofsdfsdfsdf", nil
}

func TestReport(t *testing.T) {
	Convey("When generating a report", t, func() {
		variables := url.Values{}
		variables.Add("var-test", "testvarvalue")
		gClient := &mockGrafanaClient{0, variables}
		dashboard, _ := gClient.Dashboard("")
		rep, _ := newReport(logger, gClient, &ReportOptions{
			timeRange: TimeRange{"1453206447000", "1453213647000"},
			dashUID:   "testDash",
			config:    &Config{},
			dashboard: dashboard,
		})

		Convey("When rendering images", func() {
			err := rep.renderPNGsParallel()
			So(err, ShouldBeNil)

			Convey("It should call getPanelPng once per panel", func() {
				So(gClient.getPanelCallCount, ShouldEqual, 9)
			})
		})

		Convey("When genereting the HTML files", func() {
			err := rep.renderPNGsParallel()
			So(err, ShouldBeNil)

			err = rep.generateHTMLFile()
			So(err, ShouldBeNil)

			Convey("The file should contain reference to the template data", func() {
				s := rep.options.html.body

				Convey("Including the Title", func() {
					So(rep.options.html.header, ShouldContainSubstring, "My first dashboard")

				})
				Convey("Including the variable values", func() {
					So(rep.options.html.header, ShouldContainSubstring, "testvarvalue")

				})
				Convey("and the images", func() {
					So(s, ShouldContainSubstring, "data:image/png")
					So(strings.Count(s, "data:image/png"), ShouldEqual, 9)

					So(s, ShouldContainSubstring, "image1")
					So(s, ShouldContainSubstring, "image22")
					So(s, ShouldContainSubstring, "image33")
					So(s, ShouldContainSubstring, "image44")
					So(s, ShouldContainSubstring, "image55")
					So(s, ShouldContainSubstring, "image66")
					So(s, ShouldContainSubstring, "image77")
					So(s, ShouldContainSubstring, "image88")
					So(s, ShouldContainSubstring, "image99")
				})
				Convey("and the time range", func() {
					// server time zone by shift hours timestamp
					// so just test for day and year
					So(rep.options.html.header, ShouldContainSubstring, "Tue Jan 19")
					So(rep.options.html.header, ShouldContainSubstring, "2016")
				})
			})
		})
	})

}

type errClient struct {
	getPanelCallCount int
	variables         url.Values
}

func (e *errClient) Dashboard(_ context.Context, dashName string) (Dashboard, error) {
	return NewDashboard([]byte(dashJSON), nil, e.variables, &Config{})
}

// Produce an error on the 2nd panel fetched
func (e *errClient) PanelPNG(p Panel, dashName string, t TimeRange) (string, error) {
	e.getPanelCallCount++
	if e.getPanelCallCount == 2 {
		return "", errors.New("The second panel has some problem")
	}
	return "iVBORw0KGgofsdfsdfsdf", nil
}

func TestReportErrorHandling(t *testing.T) {
	Convey("When generating a report where one panels gives an error", t, func() {
		variables := url.Values{}
		gClient := &errClient{0, variables}
		rep, _ := newReport(logger, gClient, &ReportOptions{
			timeRange: TimeRange{"1453206447000", "1453213647000"},
			dashUID:   "testDash",
			config:    &Config{Layout: "simple"},
		})

		Convey("When rendering images", func() {
			dashboard, _ := gClient.Dashboard(context.Background(), "")
			rep.options.dashboard = dashboard
			err := rep.renderPNGsParallel()

			Convey("It shoud call getPanelPng once per panel", func() {
				So(gClient.getPanelCallCount, ShouldEqual, 9)
			})

			Convey(
				"If any panels return errors, renderPNGsParallel should return the error message from one panel",
				func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "The second panel has some problem")
				},
			)
		})
	})
}
