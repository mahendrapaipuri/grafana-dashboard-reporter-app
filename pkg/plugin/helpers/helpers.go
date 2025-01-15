package helpers

import (
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"golang.org/x/mod/semver"
)

// TimeTrack tracks execution time of each function.
func TimeTrack(start time.Time, name string, logger log.Logger, args ...interface{}) {
	elapsed := time.Since(start)
	args = append(args, "duration", elapsed.String())
	logger.Debug(name, args...)
}

// SemverCompare compares the semantic version of Grafana versions.
// Grafana uses "+" as post release suffix and "-" as pre-release
// suffixes. We take that into account when calling upstream semver
// package.
func SemverCompare(a, b string) int {
	switch {
	case strings.HasPrefix(a, b+"+"):
		return 1
	case strings.HasPrefix(b, a+"+"):
		return -1
	case strings.HasPrefix(a, b+"-"):
		return -1
	case strings.HasPrefix(b, a+"-"):
		return 1
	}

	return semver.Compare(a, b)
}
