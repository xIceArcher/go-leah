package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/relvacode/iso8601"
	"github.com/rickb777/date/period"
)

func ParseISOTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}

	t, err := iso8601.ParseString(s)
	if err != nil {
		return time.Time{}, false
	}

	return t, true
}

func ParseISODuration(s string) (time.Duration, bool) {
	if s == "" {
		return time.Duration(0), false
	}

	p, err := period.Parse(s)
	if err != nil {
		return time.Duration(0), false
	}

	return p.DurationApprox(), true
}

func FormatDuration(d time.Duration) (s string) {
	totalSeconds := int(d.Seconds())

	seconds, totalMinutes := totalSeconds%60, totalSeconds/60
	minutes, totalHours := totalMinutes%60, totalMinutes/60
	hours, days := totalHours%24, totalHours/24

	components := make([]string, 0)
	if days > 0 {
		components = append(components, fmt.Sprintf("%vd", days))
	}
	if hours > 0 {
		components = append(components, fmt.Sprintf("%vh", hours))
	}
	if minutes > 0 {
		components = append(components, fmt.Sprintf("%vm", minutes))
	}
	if seconds > 0 {
		components = append(components, fmt.Sprintf("%vs", seconds))
	}

	return strings.Join(components, " ")
}

func FormatDurationSimple(d time.Duration) string {
	totalSeconds := int(d.Seconds())

	seconds, totalMinutes := totalSeconds%60, totalSeconds/60
	minutes, hours := totalMinutes%60, totalMinutes/60

	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}

func ParseHexColor(s string) int {
	colorInt, _ := strconv.ParseInt(s, 16, 0)
	return int(colorInt)
}

func Unique(ss []string) []string {
	mp := make(map[string]struct{})
	for _, s := range ss {
		mp[s] = struct{}{}
	}

	ret := make([]string, 0, len(ss))
	for s := range mp {
		ret = append(ret, s)
	}

	return ret
}
