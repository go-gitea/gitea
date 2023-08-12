// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package notify

import (
	"net/http"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
)

// ListNotifications list users's notification threads
func ListNotifications(ctx *context.APIContext) {
	// swagger:operation GET /notifications notification notifyGetList
	// ---
	// summary: List users's notification threads
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: all
	//   in: query
	//   description: If true, show notifications marked as read. Default value is false
	//   type: boolean
	// - name: status-types
	//   in: query
	//   description: "Show notifications with the provided status types. Options are: unread, read and/or pinned. Defaults to unread & pinned."
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

	totalCount, err := activities_model.CountNotifications(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	nl, err := activities_model.GetNotifications(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	err = nl.LoadAttributes(ctx)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.SetTotalCountHeader(totalCount)
	ctx.JSON(http.StatusOK, convert.ToNotifications(nl))
}

// ReadNotifications mark notification threads as read, unread, or pinned
func ReadNotifications(ctx *context.APIContext) {
	// swagger:operation PUT /notifications notification notifyReadList
	// ---
	// summary: Mark notification threads as read, pinned or unread
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: last_read_at
	//   in: query
	//   description: Describes the last point that notifications were checked. Anything updated since this time will not be updated.
	//   type: string
	//   format: date-time
	//   required: false
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
	//   description: Status to mark notifications as, Defaults to read.
	//   type: string
	//   required: false
	// responses:
	//   "205":
	//     "$ref": "#/responses/NotificationThreadList"

	lastRead := int64(0)
	qLastRead := ctx.FormTrim("last_read_at")
	if len(qLastRead) > 0 {
		tmpLastRead, err := time.Parse(time.RFC3339, qLastRead)
		if err != nil {
			ctx.Error(http.StatusBadRequest, "Parse", err)
			return
		}
		if !tmpLastRead.IsZero() {
			lastRead = tmpLastRead.Unix()
		}
	}
	opts := &activities_model.FindNotificationOptions{
		UserID:            ctx.Doer.ID,
		UpdatedBeforeUnix: lastRead,
	}
	if !ctx.FormBool("all") {
		statuses := ctx.FormStrings("status-types")
		opts.Status = statusStringsToNotificationStatuses(statuses, []string{"unread"})
	}
	nl, err := activities_model.GetNotifications(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
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
			ctx.InternalServerError(err)
			return
		}
		_ = notif.LoadAttributes(ctx)
		changed = append(changed, convert.ToNotificationThread(notif))
	}

	ctx.JSON(http.StatusResetContent, changed)
}
