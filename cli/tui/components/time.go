package components

import (
	"strconv"
	"time"
)

// FormatDuration renders a compact human-readable duration.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Second {
		return strconv.Itoa(int(d.Milliseconds())) + "ms"
	}
	if d < time.Minute {
		return strconv.FormatFloat(d.Seconds(), 'f', 1, 64) + "s"
	}
	minutes := int(d / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	return strconv.Itoa(minutes) + "m" + leftPad(strconv.Itoa(seconds), 2) + "s"
}
