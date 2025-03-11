// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

func ParseDeadlineDateToEndOfDay(date string) (timeutil.TimeStamp, error) {
	if date == "" {
		return 0, nil
	}
	deadline, err := time.ParseInLocation("2006-01-02", date, setting.DefaultUILocation)
	if err != nil {
		return 0, err
	}
	deadline = time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 23, 59, 59, 0, deadline.Location())
	return timeutil.TimeStamp(deadline.Unix()), nil
}

func ParseAPIDeadlineToEndOfDay(t *time.Time) (timeutil.TimeStamp, error) {
	if t == nil || t.IsZero() || t.Unix() == 0 {
		return 0, nil
	}
	deadline := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, setting.DefaultUILocation)
	return timeutil.TimeStamp(deadline.Unix()), nil
}
