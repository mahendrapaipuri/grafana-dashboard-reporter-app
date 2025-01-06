package helpers

import (
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// TimeTrack tracks execution time of each function.
func TimeTrack(start time.Time, name string, logger log.Logger, args ...interface{}) {
	elapsed := time.Since(start)
	args = append(args, "duration", elapsed.String())
	logger.Debug(name, args...)
}
