package plugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	. "github.com/smartystreets/goconvey/convey"
)

// mockCallResourceResponseSender implements backend.CallResourceResponseSender
// for use in tests.
type mockCallResourceResponseSender struct {
	response *backend.CallResourceResponse
}

// Send sets the received *backend.CallResourceResponse to s.response
func (s *mockCallResourceResponseSender) Send(response *backend.CallResourceResponse) error {
	s.response = response
	return nil
}

type mockReport struct {
}

func (m mockReport) Generate() (pdf io.ReadCloser, err error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (m mockReport) Clean() {}

func (m mockReport) Title() string { return "title" }

// Test report resource
func TestReportResource(t *testing.T) {
	// Set appURL env variable
	t.Setenv("GF_APP_URL", "http://localhost:3000")

	// Initialize app
	inst, err := NewApp(context.Background(), backend.AppInstanceSettings{})
	if err != nil {
		t.Fatalf("new app: %s", err)
	}
	if inst == nil {
		t.Fatal("inst must not be nil")
	}
	app, ok := inst.(*App)
	if !ok {
		t.Fatal("inst must be of type *App")
	}

	Convey("When the report handler is called", t, func() {
		var clientVars url.Values
		// mock new grafana client function to capture and validate its input parameters
		app.newGrafanaClient = func(client *http.Client, url string, cookie string, variables url.Values, gridLayout bool) GrafanaClient {
			clientVars = variables
			return NewGrafanaClient(&testClient, url, "", clientVars, false)
		}
		//mock new report function to capture and validate its input parameters
		var repDashName string
		app.newReport = func(logger log.Logger, g GrafanaClient, config *ReportConfig) Report {
			repDashName = config.dashUID
			fmt.Println(repDashName)
			return &mockReport{}
		}

		Convey("It should extract dashboard ID from the URL and forward it to the new reporter ", func() {
			var r mockCallResourceResponseSender
			err = app.CallResource(context.Background(), &backend.CallResourceRequest{
				PluginContext: backend.PluginContext{
					OrgID:    3,
					PluginID: "my-plugin",
					User:     &backend.User{Name: "foobar", Email: "foo@bar.com", Login: "foo@bar.com"},
					AppInstanceSettings: &backend.AppInstanceSettings{},
				},
				Method: http.MethodGet,
				Path:   "report?dashUid=testDash",
			}, &r)
			So(repDashName, ShouldEqual, "testDash")
		})

		Convey("It should extract the grafana variables and forward them to the new Grafana Client ", func() {
			var r mockCallResourceResponseSender
			err = app.CallResource(context.Background(), &backend.CallResourceRequest{
				PluginContext: backend.PluginContext{
					OrgID:    3,
					PluginID: "my-plugin",
					User:     &backend.User{Name: "foobar", Email: "foo@bar.com", Login: "foo@bar.com"},
					AppInstanceSettings: &backend.AppInstanceSettings{},
				},
				Method: http.MethodGet,
				Path:   "report?dashUid=testDash&var-test=testValue",
			}, &r)
			expected := url.Values{}
			expected.Add("var-test", "testValue")
			So(clientVars, ShouldResemble, expected)

			Convey("Variables should not contain other query parameters ", func() {
				var r mockCallResourceResponseSender
				err = app.CallResource(context.Background(), &backend.CallResourceRequest{
					PluginContext: backend.PluginContext{
						OrgID:    3,
						PluginID: "my-plugin",
						User:     &backend.User{Name: "foobar", Email: "foo@bar.com", Login: "foo@bar.com"},
						AppInstanceSettings: &backend.AppInstanceSettings{},
					},
					Method: http.MethodGet,
					Path:   "report?dashUid=testDash&var-test=testValue&apiToken=abcd",
				}, &r)
				expected := url.Values{}
				expected.Add("var-test", "testValue")
				So(clientVars, ShouldResemble, expected)
			})
		})
	})
}
