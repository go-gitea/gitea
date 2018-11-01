package util

import (
	"time"
)

// TimeStamp defines a timestamp
type TimeStamp int64

// TimeStampNow returns now int64
func TimeStampNow() TimeStamp {
	return TimeStamp(time.Now().Unix())
}

// Add adds seconds and return sum
func (ts TimeStamp) Add(seconds int64) TimeStamp {
	return ts + TimeStamp(seconds)
}

// AddDuration adds time.Duration and return sum
func (ts TimeStamp) AddDuration(interval time.Duration) TimeStamp {
	return ts + TimeStamp(interval/time.Second)
}

// Year returns the time's year
func (ts TimeStamp) Year() int {
	return ts.AsTime().Year()
}

// AsTime convert timestamp as time.Time in Local locale
func (ts TimeStamp) AsTime() (tm time.Time) {
	tm = time.Unix(int64(ts), 0).Local()
	return
}

// AsTimePtr convert timestamp as *time.Time in Local locale
func (ts TimeStamp) AsTimePtr() *time.Time {
	tm := time.Unix(int64(ts), 0).Local()
	return &tm
}

// Format formats timestamp as
func (ts TimeStamp) Format(f string) string {
	return ts.AsTime().Format(f)
}

// FormatLong formats as RFC1123Z
func (ts TimeStamp) FormatLong() string {
	return ts.Format(time.RFC1123Z)
}

// FormatShort formats as short
func (ts TimeStamp) FormatShort() string {
	return ts.Format("Jan 02, 2006")
}

// IsZero is zero time
func (ts TimeStamp) IsZero() bool {
	return ts.AsTime().IsZero()
}
