// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"html/template"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

type DateUtils struct {
	ctx context.Context
}

func NewDateUtils(ctx context.Context) *DateUtils {
	return &DateUtils{ctx}
}

// AbsoluteShort renders in "Jan 01, 2006" format
func (du *DateUtils) AbsoluteShort(time any) template.HTML {
	return timeutil.DateTime("short", time)
}

// AbsoluteLong renders in "January 01, 2006" format
func (du *DateUtils) AbsoluteLong(time any) template.HTML {
	return timeutil.DateTime("short", time)
}

// FullTime renders in "Jan 01, 2006 20:33:44" format
func (du *DateUtils) FullTime(time any) template.HTML {
	return timeutil.DateTime("full", time)
}

// ParseLegacy parses the datetime in legacy format, eg: "2016-01-02" in server's timezone.
// It shouldn't be used in new code. New code should use Time or TimeStamp as much as possible.
func (du *DateUtils) ParseLegacy(datetime string) time.Time {
	return parseLegacy(datetime)
}

func parseLegacy(datetime string) time.Time {
	t, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		t, _ = time.ParseInLocation(time.DateOnly, datetime, setting.DefaultUILocation)
	}
	return t
}

func dateTimeLegacy(format string, datetime any, _ ...string) template.HTML {
	if !setting.IsProd || setting.IsInTesting {
		panic("dateTimeLegacy is for backward compatibility only, do not use it in new code")
	}
	if s, ok := datetime.(string); ok {
		datetime = parseLegacy(s)
	}
	return timeutil.DateTime(format, datetime)
}
