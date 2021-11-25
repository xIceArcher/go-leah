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

func FormatDiscordRelativeTime(t time.Time) (s string) {
	return fmt.Sprintf("<t:%v:R>", t.Unix())
}

func FormatDurationSimple(d time.Duration) string {
	totalSeconds := int(d.Seconds())

	seconds, totalMinutes := totalSeconds%60, totalSeconds/60
	minutes, hours := totalMinutes%60, totalMinutes/60

	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}

func FormatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())

	seconds, totalMinutes := totalSeconds%60, totalSeconds/60
	minutes, totalHours := totalMinutes%60, totalMinutes/60
	hours, days := totalHours%24, totalHours/24

	ret := ""
	if days != 0 {
		ret += strconv.Itoa(days) + "d "
	}
	if hours != 0 {
		ret += strconv.Itoa(hours) + "h "
	}
	if minutes != 0 {
		ret += strconv.Itoa(minutes) + "m "
	}
	if seconds != 0 {
		ret += strconv.Itoa(seconds) + "s"
	}

	return strings.TrimSpace(ret)
}

func ParseHexColor(s string) int {
	colorInt, _ := strconv.ParseInt(s, 16, 0)
	return int(colorInt)
}

func Unique(ss []string) (ret []string) {
	mp := make(map[string]struct{})

	for _, s := range ss {
		if _, ok := mp[s]; !ok {
			ret = append(ret, s)
			mp[s] = struct{}{}
		}
	}

	return ret
}

func Contains(ss []string, toFind string) bool {
	for _, s := range ss {
		if s == toFind {
			return true
		}
	}

	return false
}
