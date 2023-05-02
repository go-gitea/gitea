// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// TimeStampNano defines a nano timestamp
type TimeStampNano int64

var (
	// Used for IsZero, to check if timestamp is the zero time instant.
	timeZeroUnixNano = time.Time{}.UnixNano()
)

// TimeStampNanoNow returns now nano int64
func TimeStampNanoNow() TimeStampNano {
	if !mock.IsZero() {
		return TimeStampNano(mock.UnixNano())
	}
	return TimeStampNano(time.Now().UnixNano())
}

// Add adds nanos and return sum
func (tsn TimeStampNano) Add(nanos int64) TimeStampNano {
	return tsn + TimeStampNano(nanos)
}

// AddDuration adds time.Duration and return sum
func (tsn TimeStampNano) AddDuration(interval time.Duration) TimeStampNano {
	return tsn + TimeStampNano(interval/time.Nanosecond)
}

// Year returns the time's year
func (tsn TimeStampNano) Year() int {
	return tsn.AsTime().Year()
}

// AsTime convert timestamp as time.Time in Local locale
func (tsn TimeStampNano) AsTime() (tm time.Time) {
	return tsn.AsTimeInLocation(setting.DefaultUILocation)
}

// AsLocalTime convert timestamp as time.Time in local location
func (tsn TimeStampNano) AsLocalTime() time.Time {
	return time.Unix(0, int64(tsn))
}

// AsTimeInLocation convert timestamp as time.Time in Local locale
func (tsn TimeStampNano) AsTimeInLocation(loc *time.Location) time.Time {
	return time.Unix(0, int64(tsn)).In(loc)
}

// AsTimePtr convert timestamp as *time.Time in Local locale
func (tsn TimeStampNano) AsTimePtr() *time.Time {
	return tsn.AsTimePtrInLocation(setting.DefaultUILocation)
}

// AsTimePtrInLocation convert timestamp as *time.Time in customize location
func (tsn TimeStampNano) AsTimePtrInLocation(loc *time.Location) *time.Time {
	tm := time.Unix(0, int64(tsn)).In(loc)
	return &tm
}

// Format formats timestamp as given format
func (tsn TimeStampNano) Format(f string) string {
	return tsn.FormatInLocation(f, setting.DefaultUILocation)
}

// FormatInLocation formats timestamp as given format with spiecific location
func (tsn TimeStampNano) FormatInLocation(f string, loc *time.Location) string {
	return tsn.AsTimeInLocation(loc).Format(f)
}

// FormatLong formats as RFC1123Z
func (tsn TimeStampNano) FormatLong() string {
	return tsn.Format(time.RFC1123Z)
}

// FormatShort formats as short
func (tsn TimeStampNano) FormatShort() string {
	return tsn.Format("Jan 02, 2006")
}

// FormatDate formats a date in YYYY-MM-DD server time zone
func (tsn TimeStampNano) FormatDate() string {
	return time.Unix(0, int64(tsn)).String()[:10]
}

// IsZero is zero time
func (tsn TimeStampNano) IsZero() bool {
	return int64(tsn) == 0 || int64(tsn) == timeZeroUnixNano
}
