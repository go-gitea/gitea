// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package timeutil

import (
	"time"

	"code.gitea.io/gitea/modules/setting"
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
	return ts.AsTimeInLocation(setting.DefaultUILocation)
}

// AsTimeInLocation convert timestamp as time.Time in Local locale
func (ts TimeStamp) AsTimeInLocation(loc *time.Location) (tm time.Time) {
	tm = time.Unix(int64(ts), 0).In(loc)
	return
}

// AsTimePtr convert timestamp as *time.Time in Local locale
func (ts TimeStamp) AsTimePtr() *time.Time {
	return ts.AsTimePtrInLocation(setting.DefaultUILocation)
}

// AsTimePtrInLocation convert timestamp as *time.Time in customize location
func (ts TimeStamp) AsTimePtrInLocation(loc *time.Location) *time.Time {
	tm := time.Unix(int64(ts), 0).In(loc)
	return &tm
}

// Format formats timestamp as given format
func (ts TimeStamp) Format(f string) string {
	return ts.FormatInLocation(f, setting.DefaultUILocation)
}

// FormatInLocation formats timestamp as given format with spiecific location
func (ts TimeStamp) FormatInLocation(f string, loc *time.Location) string {
	return ts.AsTimeInLocation(loc).Format(f)
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
	return ts.AsTimeInLocation(time.Local).IsZero()
}
