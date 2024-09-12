// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"strings"
	"time"
)

// GetQueryBeforeSince return parsed time (unix format) from URL query's before and since
func GetQueryBeforeSince(ctx *Base) (before, since int64, err error) {
	before, err = parseFormTime(ctx, "before")
	if err != nil {
		return 0, 0, err
	}

	since, err = parseFormTime(ctx, "since")
	if err != nil {
		return 0, 0, err
	}
	return before, since, nil
}

// parseTime parse time and return unix timestamp
func parseFormTime(ctx *Base, name string) (int64, error) {
	value := strings.TrimSpace(ctx.FormString(name))
	if len(value) != 0 {
		t, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return 0, err
		}
		if !t.IsZero() {
			return t.Unix(), nil
		}
	}
	return 0, nil
}
