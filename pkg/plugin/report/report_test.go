package report

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/worker"
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

var logger = log.NewNullLogger()

type mockGrafanaClient struct {
	getPanelCallCount int
	variables         url.Values
}

func (m *mockGrafanaClient) Dashboard(_ context.Context, _ string) (dashboard.Dashboard, error) {
	return dashboard.New(logger, config.Config{}, []byte(dashJSON), nil, m.variables)
}

func (m *mockGrafanaClient) PanelPNG(
	_ context.Context,
	_ string,
	_ dashboard.Panel,
	_ dashboard.TimeRange,
) (dashboard.PanelImage, error) {
	m.getPanelCallCount++

	return dashboard.PanelImage{Image: "iVBORw0KGgofsdfsdfsdf", MimeType: "image/png"}, nil
}

func TestReport(t *testing.T) {
	Convey("When generating a PDF", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		variables := url.Values{}
		variables.Add("var-test", "testvarvalue")
		gClient := &mockGrafanaClient{0, variables}
		workerPools := worker.Pools{
			worker.Browser:  worker.New(ctx, 6),
			worker.Renderer: worker.New(ctx, 2),
		}

		rep, err := New(logger, config.Config{}, nil, workerPools, gClient, &Options{
			TimeRange: dashboard.TimeRange{From: "1453206447000", To: "1453213647000"},
			DashUID:   "testDash",
		})
		So(err, ShouldBeNil)

		err = rep.fetchDashboard(ctx)

		Convey("It should have one", func() {
			So(errors.Is(err, dashboard.ErrNoDashboardData), ShouldBeTrue)
			So(rep.grafanaDashboard, ShouldNotBeNil)
		})

		Convey("When rendering images", func() {
			err := rep.renderPNGsParallel(ctx)
			So(err, ShouldBeNil)

			Convey("It should call getPanelPng once per panel", func() {
				So(gClient.getPanelCallCount, ShouldEqual, 9)
			})
		})

		Convey("When generating the HTML files", func() {
			err := rep.renderPNGsParallel(ctx)
			So(err, ShouldBeNil)

			err = rep.generateHTMLFile()
			So(err, ShouldBeNil)

			Convey("The file should contain reference to the template data", func() {
				s := rep.pdfOptions.Body

				Convey("Including the Title", func() {
					So(rep.pdfOptions.Header, ShouldContainSubstring, "My first dashboard")
				})
				Convey("Including the variable values", func() {
					So(rep.pdfOptions.Header, ShouldContainSubstring, "testvarvalue")
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
					So(rep.pdfOptions.Header, ShouldContainSubstring, "Tue Jan 19")
					So(rep.pdfOptions.Header, ShouldContainSubstring, "2016")
				})
			})
		})
	})
}

type errClient struct {
	getPanelCallCount int
	variables         url.Values
}

func (e *errClient) Dashboard(_ context.Context, _ string) (dashboard.Dashboard, error) {
	return dashboard.New(logger, config.Config{}, []byte(dashJSON), nil, e.variables)
}

// Produce an error on the 2nd panel fetched.
func (e *errClient) PanelPNG(
	_ context.Context,
	_ string,
	_ dashboard.Panel,
	_ dashboard.TimeRange,
) (dashboard.PanelImage, error) {
	e.getPanelCallCount++
	if e.getPanelCallCount == 2 {
		return dashboard.PanelImage{}, errors.New("the second panel has some problem")
	}

	return dashboard.PanelImage{Image: "iVBORw0KGgofsdfsdfsdf", MimeType: "image/png"}, nil
}

func TestReportErrorHandling(t *testing.T) {
	Convey("When generating a PDF where one panels gives an error", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		variables := url.Values{}
		gClient := &errClient{0, variables}
		workerPools := worker.Pools{
			worker.Browser:  worker.New(ctx, 6),
			worker.Renderer: worker.New(ctx, 2),
		}

		rep, err := New(logger, config.Config{Layout: "simple"}, nil, workerPools, gClient, &Options{
			TimeRange: dashboard.TimeRange{From: "1453206447000", To: "1453213647000"},
			DashUID:   "testDash",
		})
		So(err, ShouldBeNil)

		Convey("When rendering images", func() {
			grafanaDashboard, err := gClient.Dashboard(ctx, "")
			So(err, ShouldNotBeNil)

			rep.grafanaDashboard = grafanaDashboard
			err = rep.renderPNGsParallel(ctx)

			Convey("It should call getPanelPng once per panel", func() {
				So(gClient.getPanelCallCount, ShouldEqual, 9)
			})

			Convey(
				"If any panels return errors, renderPNGsParallel should return the error message from one panel",
				func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "the second panel has some problem")
				},
			)
		})
	})
}
