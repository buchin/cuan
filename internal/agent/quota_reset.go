package agent

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var resetsInRegex = regexp.MustCompile(`(?i)resets\s+in\s+([0-9a-zA-Z\s]+?)(?:[.,;\n\r)]|\z)`)
var daysRegex = regexp.MustCompile(`(?i)(\d+)\s*d`)
var hoursRegex = regexp.MustCompile(`(?i)(\d+)\s*h`)
var minsRegex = regexp.MustCompile(`(?i)(\d+)\s*m`)
var secsRegex = regexp.MustCompile(`(?i)(\d+)\s*s`)

// ParseProviderQuotaResetDuration parses reset time phrases like "Resets in 40h56m3s", "Resets in 63h.", "Resets in 2d5h"
// from error output and returns the parsed duration, formatted string representation, and whether parsing succeeded.
func ParseProviderQuotaResetDuration(raw string) (time.Duration, string, bool) {
	match := resetsInRegex.FindStringSubmatch(raw)
	if len(match) < 2 {
		return 0, "", false
	}
	s := strings.TrimSpace(match[1])
	if s == "" {
		return 0, "", false
	}
	var totalSeconds int64
	hasComponent := false

	if m := daysRegex.FindStringSubmatch(s); len(m) > 1 {
		if d, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			totalSeconds += d * 86400
			hasComponent = true
		}
	}
	if m := hoursRegex.FindStringSubmatch(s); len(m) > 1 {
		if h, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			totalSeconds += h * 3600
			hasComponent = true
		}
	}
	if m := minsRegex.FindStringSubmatch(s); len(m) > 1 {
		if mins, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			totalSeconds += mins * 60
			hasComponent = true
		}
	}
	if m := secsRegex.FindStringSubmatch(s); len(m) > 1 {
		if secs, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			totalSeconds += secs
			hasComponent = true
		}
	}
	if !hasComponent || totalSeconds <= 0 {
		return 0, "", false
	}
	dur := time.Duration(totalSeconds) * time.Second
	return dur, s, true
}
