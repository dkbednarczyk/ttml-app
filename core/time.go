package core

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

// ErrBadTime means text is not a time in the m:ss.mmm (h:mm:ss.mmm) format.
var ErrBadTime = errors.New("not a valid time")

var (
	shortTime = regexp.MustCompile(`^(\d{1,2}):(\d{2})\.(\d{3})$`)
	longTime  = regexp.MustCompile(`^(\d{1,3}):(\d{2}):(\d{2})\.(\d{3})$`)
)

// String writes the time as m:ss.mmm, or h:mm:ss.mmm from one hour on: the
// form the editor shows and accepts. TTML export writes hh:mm:ss.mmm.
func (ms Ms) String() string {
	total := int64(ms)
	hours, minutes := total/3_600_000, total/60_000%60
	seconds, millis := total/1000%60, total%1000
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d.%03d", hours, minutes, seconds, millis)
	}

	return fmt.Sprintf("%d:%02d.%03d", minutes, seconds, millis)
}

// ParseMs reads a time in exactly the form String writes: m:ss.mmm below an
// hour (the minutes may have a leading zero), h:mm:ss.mmm from one hour on.
func ParseMs(text string) (Ms, error) {
	var hours, minutes, seconds, millis int64
	if m := shortTime.FindStringSubmatch(text); m != nil {
		minutes, seconds, millis = atoi(m[1]), atoi(m[2]), atoi(m[3])
		if minutes >= 60 {
			return 0, fmt.Errorf("%w: from one hour on, use h:mm:ss.mmm", ErrBadTime)
		}
	} else if m := longTime.FindStringSubmatch(text); m != nil {
		hours, minutes, seconds, millis = atoi(m[1]), atoi(m[2]), atoi(m[3]), atoi(m[4])
		if hours == 0 {
			return 0, fmt.Errorf("%w: below one hour, use m:ss.mmm", ErrBadTime)
		}
		if minutes >= 60 {
			return 0, fmt.Errorf("%w: minutes must be below 60", ErrBadTime)
		}
	} else {
		return 0, fmt.Errorf("%w: use m:ss.mmm, like 1:05.250", ErrBadTime)
	}
	if seconds >= 60 {
		return 0, fmt.Errorf("%w: seconds must be below 60", ErrBadTime)
	}

	return Ms(((hours*60+minutes)*60+seconds)*1000 + millis), nil
}

// atoi converts digits already matched by a pattern.
func atoi(digits string) int64 {
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		panic(err) // unreachable: the patterns match only short digit runs
	}

	return n
}
