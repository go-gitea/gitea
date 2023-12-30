// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package notify

import (
	"fmt"
	"net/http"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/services/convert"
)

// GetThread get notification by ID
func GetThread(ctx *context.APIContext) {
	// swagger:operation GET /notifications/threads/{id} notification notifyGetThread
	// ---
	// summary: Get notification thread by ID
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of notification thread
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/NotificationThread"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	n := getThread(ctx)
	if n == nil {
		return
	}
	if err := n.LoadAttributes(ctx); err != nil && !issues_model.IsErrCommentNotExist(err) {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToNotificationThread(ctx, n))
}

// ReadThread mark notification as read by ID
func ReadThread(ctx *context.APIContext) {
	// swagger:operation PATCH /notifications/threads/{id} notification notifyReadThread
	// ---
	// summary: Mark notification thread as read by ID
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of notification thread
	//   type: string
	//   required: true
	// - name: to-status
	//   in: query
	//   description: Status to mark notifications as
	//   type: string
	//   default: read
	//   required: false
	// responses:
	//   "205":
	//     "$ref": "#/responses/NotificationThread"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	n := getThread(ctx)
	if n == nil {
		return
	}

	targetStatus := statusStringToNotificationStatus(ctx.FormString("to-status"))
	if targetStatus == 0 {
		targetStatus = activities_model.NotificationStatusRead
	}

	notif, err := activities_model.SetNotificationStatus(ctx, n.ID, ctx.Doer, targetStatus)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	if err = notif.LoadAttributes(ctx); err != nil && !issues_model.IsErrCommentNotExist(err) {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusResetContent, convert.ToNotificationThread(ctx, notif))
}

func getThread(ctx *context.APIContext) *activities_model.Notification {
	n, err := activities_model.GetNotificationByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if db.IsErrNotExist(err) {
			ctx.Error(http.StatusNotFound, "GetNotificationByID", err)
		} else {
			ctx.InternalServerError(err)
		}
		return nil
	}
	if n.UserID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "GetNotificationByID", fmt.Errorf("only user itself and admin are allowed to read/change this thread %d", n.ID))
		return nil
	}
	return n
}
