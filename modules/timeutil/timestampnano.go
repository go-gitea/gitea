// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// TimeStampNano is for nano time in database, do not use it unless there is a real requirement.
type TimeStampNano int64

// TimeStampNanoNow returns now nano int64
func TimeStampNanoNow() TimeStampNano {
	return TimeStampNano(time.Now().UnixNano())
}

// AsTime convert timestamp as time.Time in Local locale
func (tsn TimeStampNano) AsTime() (tm time.Time) {
	return tsn.AsTimeInLocation(setting.DefaultUILocation)
}

// AsTimeInLocation convert timestamp as time.Time in Local locale
func (tsn TimeStampNano) AsTimeInLocation(loc *time.Location) time.Time {
	return time.Unix(0, int64(tsn)).In(loc)
}
