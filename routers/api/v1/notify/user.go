// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package notify

import (
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/utils"
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
	//   type: string
	//   required: false
	// - name: since
	//   in: query
	//   description: Only show notifications updated after the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	//   required: false
	// - name: before
	//   in: query
	//   description: Only show notifications updated before the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	//   required: false
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/NotificationThreadList"

	before, since, err := utils.GetQueryBeforeSince(ctx)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	opts := models.FindNotificationOptions{
		ListOptions:       utils.GetListOptions(ctx),
		UserID:            ctx.User.ID,
		UpdatedBeforeUnix: before,
		UpdatedAfterUnix:  since,
	}
	qAll := strings.Trim(ctx.Query("all"), " ")
	if qAll != "true" {
		opts.Status = models.NotificationStatusUnread
	}
	nl, err := models.GetNotifications(opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	err = nl.LoadAttributes()
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, nl.APIFormat())
}

// ReadNotifications mark notification threads as read
func ReadNotifications(ctx *context.APIContext) {
	// swagger:operation PUT /notifications notification notifyReadList
	// ---
	// summary: Mark notification threads as read
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
	// responses:
	//   "205":
	//     "$ref": "#/responses/empty"

	lastRead := int64(0)
	qLastRead := strings.Trim(ctx.Query("last_read_at"), " ")
	if len(qLastRead) > 0 {
		tmpLastRead, err := time.Parse(time.RFC3339, qLastRead)
		if err != nil {
			ctx.InternalServerError(err)
			return
		}
		if !tmpLastRead.IsZero() {
			lastRead = tmpLastRead.Unix()
		}
	}
	opts := models.FindNotificationOptions{
		UserID:            ctx.User.ID,
		UpdatedBeforeUnix: lastRead,
		Status:            models.NotificationStatusUnread,
	}
	nl, err := models.GetNotifications(opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	for _, n := range nl {
		err := models.SetNotificationStatus(n.ID, ctx.User, models.NotificationStatusRead)
		if err != nil {
			ctx.InternalServerError(err)
			return
		}
		ctx.Status(http.StatusResetContent)
	}

	ctx.Status(http.StatusResetContent)
}
