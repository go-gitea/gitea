// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"html/template"

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
