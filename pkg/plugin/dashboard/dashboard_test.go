package dashboard

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/mahendrapaipuri/grafana-dashboard-reporter-app/pkg/plugin/config"
	. "github.com/smartystreets/goconvey/convey"
)

var logger = log.NewNullLogger()

func TestDashboard(t *testing.T) {
	Convey("When creating a new dashboard from Grafana dashboard JSON and browser data", t, func() {
		const dashJSON = `
{"dashboard":
	{
		"panels":
			[{"type":"singlestat", "id":0},
			{"type":"graph", "id":1, "gridPos":{"H":6,"W":24,"X":0,"Y":0}},
			{"type":"singlestat", "id":2, "title":"Panel3Title #"},
			{"type":"text", "gridPos":{"H":6.5,"W":20.5,"X":0,"Y":0}, "id":3},
			{"type":"table", "id":4},
			{"type":"row", "id":5, "collapsed": true}],
		"title":"DashTitle #"
	},

"Meta":
	{"Slug":"testDash"}
}`

		dashDataString := `[{"width":940,"height":258,"x":0,"y":0,"id":"12"},{"width":940,"height":258,"x":940,"y":0,"id":"26"},{"width":940,"height":258,"x":0,"y":0,"id":"27"}]`

		var dashData []interface{}
		if err := json.Unmarshal([]byte(dashDataString), &dashData); err != nil {
			t.Errorf("failed to unmarshal data: %s", err)
		}

		dash, _ := New(logger, config.Config{}, []byte(dashJSON), dashData, url.Values{})

		Convey("Panels should contain all panels from dashboard browser data", func() {
			So(dash.Panels, ShouldHaveLength, 3)
			So(dash.Panels[0].ID, ShouldEqual, "12")
			So(dash.Panels[1].ID, ShouldEqual, "26")
			So(dash.Panels[2].ID, ShouldEqual, "27")
		})
	})

	Convey("When creating a new dashboard from only Grafana dashboard JSON", t, func() {
		const dashJSON = `
{"dashboard":
	{
		"panels":
			[{"type":"singlestat", "id":0},
			{"type":"graph", "id":1, "gridPos":{"H":6,"W":24,"X":0,"Y":0}},
			{"type":"singlestat", "id":2, "title":"Panel3Title #"},
			{"type":"text", "gridPos":{"H":6.5,"W":20.5,"X":0,"Y":0}, "id":3},
			{"type":"table", "id":4},
			{"type":"row", "id":5, "collapsed": true}],
		"title":"DashTitle #"
	},

"Meta":
	{"Slug":"testDash"}
}`

		dashDataString := `[{"width":"940px","height":"258px","transform":"translate(0px)","id":"12"},{"width":"940px","height":"258px","transform":"translate(948px, 0px)","id":"26"},{"width":"940px","height":"258px","transform":"translate(0px, 266px)","id":"27"}]`

		var dashData []interface{}
		if err := json.Unmarshal([]byte(dashDataString), &dashData); err != nil {
			t.Errorf("failed to unmarshal data: %s", err)
		}

		dash, _ := New(logger, config.Config{}, []byte(dashJSON), dashData, url.Values{})

		Convey("Panels should contain all panels from dashboard JSON model", func() {
			So(dash.Panels, ShouldHaveLength, 5)
			So(dash.Panels[0].ID, ShouldEqual, "0")
			So(dash.Panels[1].ID, ShouldEqual, "1")
			So(dash.Panels[2].ID, ShouldEqual, "2")
		})
	})
}

func TestVariableValues(t *testing.T) {
	Convey("When creating a dashboard and passing url varialbes in", t, func() {
		const v5DashJSON = `
{
	"dashboard": {}
}`

		vars := url.Values{}
		vars.Add("var-one", "oneval")
		vars.Add("var-two", "twoval")
		dash, _ := New(logger, config.Config{}, []byte(v5DashJSON), nil, vars)

		Convey("The dashboard should contain the variable values in a random order", func() {
			So(dash.VariableValues, ShouldContainSubstring, "oneval")
			So(dash.VariableValues, ShouldContainSubstring, "twoval")
		})
	})
}
