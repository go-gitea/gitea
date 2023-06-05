// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package notify

import (
	"net/http"
	"strings"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// NewAvailable check if unread notifications exist
func NewAvailable(ctx *context.APIContext) {
	// swagger:operation GET /notifications/new notification notifyNewAvailable
	// ---
	// summary: Check if unread notifications exist
	// responses:
	//   "200":
	//     "$ref": "#/responses/NotificationCount"
	ctx.JSON(http.StatusOK, api.NotificationCount{New: activities_model.CountUnread(ctx, ctx.Doer.ID)})
}

func getFindNotificationOptions(ctx *context.APIContext) *activities_model.FindNotificationOptions {
	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "GetQueryBeforeSince", err)
		return nil
	}
	opts := &activities_model.FindNotificationOptions{
		ListOptions:       utils.GetListOptions(ctx),
		UserID:            ctx.Doer.ID,
		UpdatedBeforeUnix: before,
		UpdatedAfterUnix:  since,
	}
	if !ctx.FormBool("all") {
		statuses := ctx.FormStrings("status-types")
		opts.Status = statusStringsToNotificationStatuses(statuses, []string{"unread", "pinned"})
	}

	subjectTypes := ctx.FormStrings("subject-type")
	if len(subjectTypes) != 0 {
		opts.Source = subjectToSource(subjectTypes)
	}

	return opts
}

func subjectToSource(value []string) (result []activities_model.NotificationSource) {
	for _, v := range value {
		switch strings.ToLower(v) {
		case "issue":
			result = append(result, activities_model.NotificationSourceIssue)
		case "pull":
			result = append(result, activities_model.NotificationSourcePullRequest)
		case "commit":
			result = append(result, activities_model.NotificationSourceCommit)
		case "repository":
			result = append(result, activities_model.NotificationSourceRepository)
		}
	}
	return result
}
