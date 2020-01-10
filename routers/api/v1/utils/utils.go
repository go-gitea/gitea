// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
)

// UserID user ID of authenticated user, or 0 if not authenticated
func UserID(ctx *context.APIContext) int64 {
	if ctx.User == nil {
		return 0
	}
	return ctx.User.ID
}

// GetQueryBeforeSince return parsed time (unix format) from URL query's before and since
func GetQueryBeforeSince(ctx *context.APIContext) (before, since int64, err error) {
	qCreatedBefore := strings.Trim(ctx.Query("before"), " ")
	if qCreatedBefore != "" {
		createdBefore, err := time.Parse(time.RFC3339, qCreatedBefore)
		if err != nil {
			return 0, 0, err
		}
		if !createdBefore.IsZero() {
			before = createdBefore.Unix()
		}
	}

	qCreatedAfter := strings.Trim(ctx.Query("since"), " ")
	if qCreatedAfter != "" {
		createdAfter, err := time.Parse(time.RFC3339, qCreatedAfter)
		if err != nil {
			return 0, 0, err
		}
		if !createdAfter.IsZero() {
			since = createdAfter.Unix()
		}
	}
	return before, since, nil
}

// GetListOptions returns list options using the page and limit parameters
func GetListOptions(ctx *context.APIContext) models.ListOptions {
	return models.ListOptions{
		Page:     ctx.QueryInt("page"),
		PageSize: convert.ToCorrectPageSize(ctx.QueryInt("limit")),
	}
}
