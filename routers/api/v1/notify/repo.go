// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package notify

import (
	"net/http"
	"strings"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

func statusStringToNotificationStatus(status string) activities_model.NotificationStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "unread":
		return activities_model.NotificationStatusUnread
	case "read":
		return activities_model.NotificationStatusRead
	case "pinned":
		return activities_model.NotificationStatusPinned
	default:
		return 0
	}
}

func statusStringsToNotificationStatuses(statuses, defaultStatuses []string) []activities_model.NotificationStatus {
	if len(statuses) == 0 {
		statuses = defaultStatuses
	}
	results := make([]activities_model.NotificationStatus, 0, len(statuses))
	for _, status := range statuses {
		notificationStatus := statusStringToNotificationStatus(status)
		if notificationStatus > 0 {
			results = append(results, notificationStatus)
		}
	}
	return results
}

// ListRepoNotifications list users's notification threads on a specific repo
func ListRepoNotifications(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/notifications notification notifyGetRepoList
	// ---
	// summary: List users's notification threads on a specific repo
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: all
	//   in: query
	//   description: If true, show notifications marked as read. Default value is false
	//   type: boolean
	// - name: status-types
	//   in: query
	//   description: "Show notifications with the provided status types. Options are: unread, read and/or pinned. Defaults to unread & pinned"
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: subject-type
	//   in: query
	//   description: "filter notifications by subject type"
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [issue,pull,commit,repository]
	// - name: since
	//   in: query
	//   description: Only show notifications updated after the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	// - name: before
	//   in: query
	//   description: Only show notifications updated before the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/NotificationThreadList"
	opts := getFindNotificationOptions(ctx)
	if ctx.Written() {
		return
	}
	opts.RepoID = ctx.Repo.Repository.ID

	totalCount, err := db.Count[activities_model.Notification](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	nl, err := db.Find[activities_model.Notification](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	err = activities_model.NotificationList(nl).LoadAttributes(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.SetTotalCountHeader(totalCount)

	ctx.JSON(http.StatusOK, convert.ToNotifications(ctx, nl))
}

// ReadRepoNotifications mark notification threads as read on a specific repo
func ReadRepoNotifications(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/notifications notification notifyReadRepoList
	// ---
	// summary: Mark notification threads as read, pinned or unread on a specific repo
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: all
	//   in: query
	//   description: If true, mark all notifications on this repo. Default value is false
	//   type: string
	//   required: false
	// - name: status-types
	//   in: query
	//   description: "Mark notifications with the provided status types. Options are: unread, read and/or pinned. Defaults to unread."
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//   required: false
	// - name: to-status
	//   in: query
	//   description: Status to mark notifications as. Defaults to read.
	//   type: string
	//   required: false
	// - name: last_read_at
	//   in: query
	//   description: Describes the last point that notifications were checked. Anything updated since this time will not be updated.
	//   type: string
	//   format: date-time
	//   required: false
	// responses:
	//   "205":
	//     "$ref": "#/responses/NotificationThreadList"

	lastRead := int64(0)
	qLastRead := ctx.FormTrim("last_read_at")
	if len(qLastRead) > 0 {
		tmpLastRead, err := time.Parse(time.RFC3339, qLastRead)
		if err != nil {
			ctx.APIError(http.StatusBadRequest, err)
			return
		}
		if !tmpLastRead.IsZero() {
			lastRead = tmpLastRead.Unix()
		}
	}

	opts := &activities_model.FindNotificationOptions{
		UserID:            ctx.Doer.ID,
		RepoID:            ctx.Repo.Repository.ID,
		UpdatedBeforeUnix: lastRead,
	}

	if !ctx.FormBool("all") {
		statuses := ctx.FormStrings("status-types")
		opts.Status = statusStringsToNotificationStatuses(statuses, []string{"unread"})
	}
	nl, err := db.Find[activities_model.Notification](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	targetStatus := statusStringToNotificationStatus(ctx.FormString("to-status"))
	if targetStatus == 0 {
		targetStatus = activities_model.NotificationStatusRead
	}

	changed := make([]*structs.NotificationThread, 0, len(nl))

	for _, n := range nl {
		notif, err := activities_model.SetNotificationStatus(ctx, n.ID, ctx.Doer, targetStatus)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		_ = notif.LoadAttributes(ctx)
		changed = append(changed, convert.ToNotificationThread(ctx, notif))
	}
	ctx.JSON(http.StatusResetContent, changed)
}
