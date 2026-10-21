// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"time"

	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
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

func ApplyAPIDeadlineTimestamp(field *timeutil.TimeStamp, input optional.Option[*string], remove *bool) (changed bool, err error) {
	old := *field
	if remove != nil && *remove || input.Has() && (input.Value() == nil || *input.Value() == "") {
		*field = 0
		return old != *field, nil
	}

	if !input.Has() {
		return false, nil
	}
	inputValue := *input.Value()

	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}
	for _, format := range formats {
		t, err := time.ParseInLocation(format, inputValue, setting.DefaultUILocation)
		if err == nil {
			ts := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, setting.DefaultUILocation).Unix()
			if ts < 86400*2 {
				ts = 0 // tolerate timezone difference between server and client
			}
			*field = timeutil.TimeStamp(ts)
			return old != *field, nil
		}
	}
	return false, util.NewInvalidArgumentErrorf("invalid date format: %s", inputValue)
}
