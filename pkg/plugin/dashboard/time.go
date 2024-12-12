package dashboard

import (
	"regexp"
	"strconv"
	"time"
)

type TimeRange struct {
	From string
	To   string
}

// Used to parse grafana time specifications. These can take various forms:
//   - relative: "now", "now-1h", "now-2d", "now-3w", "now-5M", "now-1y"
//   - human friendly boundary:
//     From:"now/d" -> start of today
//     To:  "now/d" -> end of today
//     To:  "now/w" -> end of the week
//     To:  "now-1d/d" -> end of yesterday
//     When used as boundary, the same string will evaluate to a different time if used in 'From' or 'To'
//   - absolute unix time: "142321234"
//   - absolute time string: "2024-12-02T23:00:00.000Z" start from Grafana v11.3.0
//
// The required behaviour is clearly documented in the unit tests, time_test.go.
type now time.Time

type boundary int

const (
	From boundary = iota
	To
)

const (
	relTimeRegExp      = "^now([+-][0-9]+)([mhdwMy])$"
	boundaryTimeRegExp = "^(.*?)/([dwMy])$"
	layout             = "2006-01-02T15:04:05.000Z"
)

// Convenience function to raise panic with custom message.
func unrecognized(s string) string {
	return s + " is not a recognised time format"
}

// Add time duration based on boundary.
func add(b boundary) int {
	if b == To {
		return 1
	}
	// b == From
	return 0
}

// Convert days to week boundary.
func daysToWeekBoundary(wd time.Weekday, b boundary) int {
	if b == To {
		return 1 + int(time.Saturday) - int(wd)
	} else {
		// b == From
		return -int(wd)
	}
}

// Parse grafana specific time to time.Time format.
func roundTimeToBoundary(t time.Time, b boundary, boundaryUnit string) time.Time {
	y := t.Year()
	M := t.Month()
	d := t.Day()

	switch boundaryUnit {
	case "d":
		d += add(b)
	case "w":
		d += daysToWeekBoundary(t.Weekday(), b)
	case "M":
		d = 1
		M = time.Month(int(M) + add(b))
	case "y":
		d = 1
		M = time.January
		y += add(b)
	}

	return time.Date(y, M, d, 0, 0, 0, 0, t.Location())
}

// Parse time stamp to time.Unix() format.
func parseAbsTime(s string) time.Time {
	// Check if time is in unix timestamp format
	if timeInMs, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(timeInMs/1000, 0)
	}

	// Check if time is in 2024-12-02T23:00:00.000Z format
	if absTime, err := time.Parse(layout, s); err == nil && absTime.Unix() > 0 {
		return absTime
	}

	panic(unrecognized(s))
}

// If time string is relative.
func isRelativeTime(s string) bool {
	matched, _ := regexp.MatchString(relTimeRegExp, s)

	return matched
}

// If time string is in boundary expression.
func isHumanFriendlyBoundray(s string) bool {
	matched, _ := regexp.MatchString(boundaryTimeRegExp, s)

	return matched
}

// NewTimeRange creates a new TimeRange struct.
func NewTimeRange(from, to string) TimeRange {
	if from == "" {
		from = "now-1h"
	}

	if to == "" {
		to = "now"
	}

	return TimeRange{from, to}
}

// Formats Grafana 'From' time spec into absolute printable time.
func (tr TimeRange) FromFormatted(loc *time.Location, layout string) string {
	n := newNow()

	return n.parseFrom(tr.From).In(loc).Format(layout)
}

// Formats Grafana 'To' time spec into absolute printable time.
func (tr TimeRange) ToFormatted(loc *time.Location, layout string) string {
	n := newNow()

	return n.parseTo(tr.To).In(loc).Format(layout)
}

// Make current time custom struct.
func newNow() now {
	return now(time.Now())
}

// Get current time as time.Time format.
func (n now) asTime() time.Time {
	return time.Time(n)
}

// Parse from time string.
func (n now) parseFrom(s string) time.Time {
	return n.parseHumanFriendlyBoundary(s, From)
}

// Parse to time string.
func (n now) parseTo(s string) time.Time {
	return n.parseHumanFriendlyBoundary(s, To)
}

// Parse time and boundary unit.
func (n now) parseTimeAndBoundaryUnit(s string) (time.Time, string) {
	re := regexp.MustCompile(boundaryTimeRegExp)

	matches := re.FindStringSubmatch(s)
	if len(matches) != 3 {
		panic(unrecognized(s))
	}

	moment := n.parseTime(matches[1])
	boundaryUnit := matches[2]

	return moment, boundaryUnit
}

// Parse boundary time string.
func (n now) parseHumanFriendlyBoundary(s string, b boundary) time.Time {
	if !isHumanFriendlyBoundray(s) {
		return n.parseTime(s)
	} else {
		moment, boundaryUnit := n.parseTimeAndBoundaryUnit(s)

		return roundTimeToBoundary(moment, b, boundaryUnit)
	}
}

// Parse time string to time.Time format.
func (n now) parseTime(s string) time.Time {
	if s == "now" {
		return n.asTime()
	} else if isRelativeTime(s) {
		return n.parseRelativeTime(s)
	} else {
		return parseAbsTime(s)
	}
}

// Parse relative time string to time.Time.
func (n now) parseRelativeTime(s string) time.Time {
	re := regexp.MustCompile(relTimeRegExp)

	matches := re.FindStringSubmatch(s)
	if len(matches) != 3 {
		panic(unrecognized(s))
	}

	unit := matches[2]
	number := matches[1]

	i, err := strconv.Atoi(number)
	if err != nil {
		panic(err)
	}

	switch unit {
	case "m", "h":
		d, err := time.ParseDuration(number + unit)
		if err != nil {
			panic(err)
		}

		return n.asTime().Add(d)
	case "d":
		return n.asTime().AddDate(0, 0, i)
	case "w":
		return n.asTime().AddDate(0, 0, i*7)
	case "M":
		return n.asTime().AddDate(0, i, 0)
	case "y":
		return n.asTime().AddDate(i, 0, 0)
	}

	return n.asTime()
}
