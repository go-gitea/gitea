// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// TimeStamp defines a timestamp
type TimeStamp int64

var (
	// mockNow is NOT concurrency-safe!!
	mockNow time.Time

	// Used for IsZero, to check if timestamp is the zero time instant.
	timeZeroUnix = time.Time{}.Unix()
)

// MockSet sets the time to a mocked time.Time
func MockSet(now time.Time) func() {
	mockNow = now
	return MockUnset
}

// MockUnset will unset the mocked time.Time
func MockUnset() {
	mockNow = time.Time{}
}

// TimeStampNow returns now int64
func TimeStampNow() TimeStamp {
	if !mockNow.IsZero() {
		return TimeStamp(mockNow.Unix())
	}
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

// AsLocalTime convert timestamp as time.Time in local location
func (ts TimeStamp) AsLocalTime() time.Time {
	return time.Unix(int64(ts), 0)
}

// AsTimeInLocation convert timestamp as time.Time in Local locale
func (ts TimeStamp) AsTimeInLocation(loc *time.Location) time.Time {
	return time.Unix(int64(ts), 0).In(loc)
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

// FormatDate formats a date in YYYY-MM-DD
func (ts TimeStamp) FormatDate() string {
	return ts.Format("2006-01-02")
}

// IsZero is zero time
func (ts TimeStamp) IsZero() bool {
	return int64(ts) == 0 || int64(ts) == timeZeroUnix
}
