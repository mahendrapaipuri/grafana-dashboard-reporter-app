package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
	. "github.com/smartystreets/goconvey/convey"
)

// mockCallResourceResponseSender implements backend.CallResourceResponseSender
// for use in tests.
type mockCallResourceResponseSender struct {
	response *backend.CallResourceResponse
}

// Send sets the received *backend.CallResourceResponse to s.response.
func (s *mockCallResourceResponseSender) Send(response *backend.CallResourceResponse) error {
	s.response = response

	return nil
}

// Test report resource.
func TestReportResource(t *testing.T) {
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

	// Initialize app
	inst, err := NewDashboardReporterApp(context.Background(), backend.AppInstanceSettings{
		DecryptedSecureJSONData: map[string]string{
			config.SaToken: "token",
		},
	})
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
		Convey("It should extract dashboard ID from the URL and forward it to the new reporter ", func() {
			var repDashName string

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/dashboards/") {
					urlParts := strings.Split(r.URL.Path, "/")
					repDashName = urlParts[len(urlParts)-1]
				}

				if _, err := w.Write([]byte(`{"dashboard": {"title": "foo","panels":[{"type":"singlestat", "id":0}]}}`)); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)

					return
				}
			}))
			defer ts.Close()

			ctx := backend.WithGrafanaConfig(context.Background(), backend.NewGrafanaCfg(map[string]string{
				backend.AppURL: ts.URL,
			}))

			var r mockCallResourceResponseSender
			err = app.CallResource(ctx, &backend.CallResourceRequest{
				PluginContext: backend.PluginContext{
					OrgID:    3,
					PluginID: "my-plugin",
					User:     &backend.User{Name: "foobar", Email: "foo@bar.com", Login: "foo@bar.com"},
				},
				Method: http.MethodGet,
				Path:   "report?dashUid=testDash",
			}, &r)

			So(repDashName, ShouldEqual, "testDash")
		})
	})
}

func TestSemverComapre(t *testing.T) {
	Convey("When comparing semantic versions", t, func() {
		tests := []struct {
			name     string
			a, b     string
			expected int
		}{
			{
				name:     "regular sem ver comparison",
				a:        "v1.2.3",
				b:        "v1.2.5",
				expected: -1,
			},
			{
				name:     "regular sem ver with pre-release comparison",
				a:        "v1.2.3",
				b:        "v1.2.3-rc0",
				expected: 1,
			},
			{
				name:     "regular sem ver with pre-release comparison with inverse order",
				a:        "v1.2.3-rc1",
				b:        "v1.2.3",
				expected: -1,
			},
			{
				name:     "regular sem ver with post-release comparison",
				a:        "v1.2.3",
				b:        "v1.2.3+security-01",
				expected: -1,
			},
			{
				name:     "regular sem ver with post-release comparison with inverse order",
				a:        "v1.2.3+security-01",
				b:        "v1.2.3",
				expected: 1,
			},
			{
				name:     "comparison with zero version",
				a:        "v0.0.0",
				b:        "v1.2.5",
				expected: -1,
			},
			{
				name:     "comparison with zero version with inverse order",
				a:        "v1.2.3",
				b:        "v0.0.0",
				expected: 1,
			},
		}

		for _, test := range tests {
			got := semverCompare(test.a, test.b)
			So(got, ShouldEqual, test.expected)
		}
	})
}
