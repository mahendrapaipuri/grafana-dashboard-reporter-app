package report

import (
	"testing"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/dashboard"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPanelSelector(t *testing.T) {
	Convey("When selecting panels based on integer panel IDs", t, func() {
		allPanels := []dashboard.Panel{
			{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "15"}, {ID: "26"}, {ID: "37"},
		}
		cases := map[string]struct {
			IncludeIDs, ExcludeIDs []string
			DefaultInclude         bool
			Result                 []int
		}{
			"empty_true": {
				nil,
				nil,
				true,
				[]int{0, 1, 2, 3, 4, 5, 6},
			},
			"empty_false": {
				nil,
				nil,
				false,
				nil,
			},
			"include": {
				[]string{"1", "4", "3"},
				nil,
				true,
				[]int{0, 2, 3},
			},
			"exclude": {
				nil,
				[]string{"2", "4", "3"},
				true,
				[]int{0, 4, 5, 6},
			},
			"exclude_false": {
				nil,
				[]string{"2", "4", "3"},
				false,
				nil,
			},
			"include_and_exclude_1": {
				[]string{"1", "4", "6"},
				[]string{"2", "4", "3"},
				true,
				[]int{0},
			},
			"include_and_exclude_2": {
				[]string{"1"},
				[]string{"2"},
				true,
				[]int{0},
			},
		}

		for clName, cl := range cases {
			filteredPanels := selectPanels(allPanels, cl.IncludeIDs, cl.ExcludeIDs, cl.DefaultInclude)

			Convey("Panels should be properly selected: "+clName, func() {
				So(filteredPanels, ShouldResemble, cl.Result)
			})
		}
	})

	// For Grafana >= v11.3.0
	Convey("When filtering png panels based on string panel IDs", t, func() {
		allPanels := []dashboard.Panel{
			{ID: "panel-1-clone-0"}, {ID: "panel-1-clone-1"}, {ID: "panel-3"}, {ID: "panel-4"}, {ID: "panel-5"}, {ID: "panel-6"}, {ID: "panel-7"}, {ID: "panel-12-clone-0"},
		}
		cases := map[string]struct {
			IncludeIDs, ExcludeIDs []string
			DefaultInclude         bool
			Result                 []int
		}{
			"include": {
				[]string{"panel-1", "panel-4", "panel-6"},
				nil,
				true,
				[]int{0, 1, 3, 5},
			},
			"exclude": {
				nil,
				[]string{"panel-5", "panel-4", "panel-3"},
				true,
				[]int{0, 1, 5, 6, 7},
			},
			"exclude_false": {
				nil,
				[]string{"panel-1", "panel-4", "panel-3"},
				false,
				nil,
			},
			"include_and_exclude": {
				[]string{"panel-1", "panel-4", "panel-6"},
				[]string{"panel-2", "panel-4", "panel-3"},
				true,
				[]int{0, 1, 5},
			},
		}

		for clName, cl := range cases {
			filteredPanels := selectPanels(allPanels, cl.IncludeIDs, cl.ExcludeIDs, cl.DefaultInclude)

			Convey("Panels should be properly filtered: "+clName, func() {
				So(filteredPanels, ShouldResemble, cl.Result)
			})
		}
	})
}
