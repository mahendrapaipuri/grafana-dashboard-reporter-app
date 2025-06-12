package helpers

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

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
			got := SemverCompare(test.a, test.b)
			So(got, ShouldEqual, test.expected)
		}
	})
}
