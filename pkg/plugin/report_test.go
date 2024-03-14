package plugin

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"os"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/afero"
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

func (m *mockGrafanaClient) GetDashboard(dashName string) (Dashboard, error) {
	return NewDashboard([]byte(dashJSON), m.variables), nil
}

func (m *mockGrafanaClient) GetPanelPNG(p Panel, dashName string, t TimeRange) (io.ReadCloser, error) {
	m.getPanelCallCount++
	return io.NopCloser(bytes.NewBuffer([]byte("Not actually a png"))), nil
}

func TestReport(t *testing.T) {
	Convey("When generating a report", t, func() {
		variables := url.Values{}
		variables.Add("var-test", "testvarvalue")
		gClient := &mockGrafanaClient{0, variables}
		rep, _ := newReport(logger, gClient, &ReportConfig{
			timeRange:        TimeRange{"1453206447000", "1453213647000"},
			dashUID:          "testDash",
			layout:           "simple",
			vfs:              afero.NewBasePathFs(afero.NewOsFs(), t.TempDir()).(*afero.BasePathFs),
			maxRenderWorkers: 2,
		})
		defer rep.Clean()

		Convey("When rendering images", func() {
			dashboard, _ := gClient.GetDashboard("")
			rep.renderPNGsParallel(dashboard)

			Convey("It should create a temporary folder", func() {
				_, err := rep.cfg.vfs.Stat(rep.cfg.stagingDir)
				So(err, ShouldBeNil)
			})

			Convey("It should copy the file to the image folder", func() {
				_, err := rep.cfg.vfs.Stat(rep.imgDirPath() + "/image1.png")
				So(err, ShouldBeNil)
			})

			Convey("It shoud call getPanelPng once per panel", func() {
				So(gClient.getPanelCallCount, ShouldEqual, 9)
			})

			Convey("It should create one file per panel", func() {
				f, _ := rep.cfg.vfs.Open(rep.imgDirPath())
				defer f.Close()
				files, err := f.Readdir(0)
				So(files, ShouldHaveLength, 9)
				So(err, ShouldBeNil)
			})
		})

		Convey("When genereting the HTML files", func() {
			dashboard, _ := gClient.GetDashboard("")
			rep.generateHTMLFile(dashboard)
			f, err := rep.cfg.vfs.Open(rep.htmlPath())
			defer f.Close()

			Convey("It should create a file in the temporary folder", func() {
				So(err, ShouldBeNil)
			})

			Convey("The file should contain reference to the template data", func() {
				var buf bytes.Buffer
				io.Copy(&buf, f)
				s := buf.String()

				So(err, ShouldBeNil)
				Convey("Including the Title", func() {
					So(rep.cfg.header, ShouldContainSubstring, "My first dashboard")

				})
				Convey("Including the varialbe values", func() {
					So(rep.cfg.header, ShouldContainSubstring, "testvarvalue")

				})
				Convey("and the images", func() {
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
					//server time zone by shift hours timestamp
					//so just test for day and year
					So(rep.cfg.header, ShouldContainSubstring, "Tue Jan 19")
					So(rep.cfg.header, ShouldContainSubstring, "2016")
				})
			})
		})

		Convey("Clean() should remove the temporary folder", func() {
			rep.Clean()

			_, err := rep.cfg.vfs.Stat(rep.cfg.stagingDir)
			So(os.IsNotExist(err), ShouldBeTrue)
		})
	})

}

type errClient struct {
	getPanelCallCount int
	variables         url.Values
}

func (e *errClient) GetDashboard(dashName string) (Dashboard, error) {
	return NewDashboard([]byte(dashJSON), e.variables), nil
}

// Produce an error on the 2nd panel fetched
func (e *errClient) GetPanelPNG(p Panel, dashName string, t TimeRange) (io.ReadCloser, error) {
	e.getPanelCallCount++
	if e.getPanelCallCount == 2 {
		return nil, errors.New("The second panel has some problem")
	}
	return io.NopCloser(bytes.NewBuffer([]byte("Not actually a png"))), nil
}

func TestReportErrorHandling(t *testing.T) {
	Convey("When generating a report where one panels gives an error", t, func() {
		variables := url.Values{}
		gClient := &errClient{0, variables}
		rep, _ := newReport(logger, gClient, &ReportConfig{
			timeRange: TimeRange{"1453206447000", "1453213647000"},
			vfs:       afero.NewBasePathFs(afero.NewOsFs(), t.TempDir()).(*afero.BasePathFs),
			dashUID:   "testDash",
			layout:    "simple",
		})
		defer rep.Clean()

		Convey("When rendering images", func() {
			dashboard, _ := gClient.GetDashboard("")
			err := rep.renderPNGsParallel(dashboard)

			Convey("It shoud call getPanelPng once per panel", func() {
				So(gClient.getPanelCallCount, ShouldEqual, 9)
			})

			Convey("It should create one less image file than the total number of panels", func() {
				f, _ := rep.cfg.vfs.Open(rep.imgDirPath())
				defer f.Close()
				files, err := f.Readdir(0)
				So(files, ShouldHaveLength, 8) // one less than the total number of im
				So(err, ShouldBeNil)
			})

			Convey("If any panels return errors, renderPNGsParralel should return the error message from one panel", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "The second panel has some problem")
			})
		})

		Convey("Clean() should remove the temporary folder", func() {
			rep.Clean()

			_, err := rep.cfg.vfs.Stat(rep.cfg.stagingDir)
			So(os.IsNotExist(err), ShouldBeTrue)
		})
	})
}
