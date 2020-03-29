// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package notify

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

// NewAvailable check if unread notifications exist
func NewAvailable(ctx *context.APIContext) {
	// swagger:operation GET /notifications/new notification notifyNewAvailable
	// ---
	// summary: Check if unread notifications exist
	// responses:
	//   "200":
	//    "$ref": "#/responses/NotificationCount"
	//   "204":
	//     description: No unread notification

	count := models.CountUnread(ctx.User)

	if count > 0 {
		ctx.JSON(http.StatusOK, api.NotificationCount{New: count})
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

// NewWebPushSubscription check if unread notifications exist
func NewWebPushSubscription(ctx *context.APIContext, input api.NotificationWebPushSubscription) {
	// swagger:operation POST /notifications/subscription notification notifyNewWebPushSubscription
	// ---
	// summary: Create a Web Push subscription for the current user to receieve push notifications.
	//          This will also produce a test notification to ensure the given details are valid.
	// consumes:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/NotificationWebPushSubscription"
	// responses:
	//   "201":
	//     description: The Web Push subscription was tested and saved sucessfully.
	//   "422":
	//     description: Required fields were missing or the provided subscription could not be tested successfully.

	if input.Endpoint == "" || input.Auth == "" || input.P256DH == "" {
		ctx.Status(http.StatusUnprocessableEntity)
		return
	}

	testPayload := &api.NotificationPayload{
		Title: setting.AppName,
		Text:  "This is a test notification from Gitea.",
		URL:   setting.AppSubURL,
	}
	resp, err := models.SendWebPushNotification(&input, testPayload)
	if err != nil {
		// An invalid key causes a mathematical error. This is the user's fault.
		if strings.Contains(err.Error(), "key is not a valid point on the curve") {
			ctx.Status(http.StatusUnprocessableEntity)
			return
		}
		// Otherwise it could be a network problem.
		ctx.Status(http.StatusInternalServerError)
		return
	}

	// Web Push returns 201 on success.
	if resp.StatusCode != http.StatusCreated {
		ctx.Status(http.StatusUnprocessableEntity)
		return
	}

	err = models.CreateWebPushSubscription(ctx.User.ID, &input)
	if err != nil {
		log.Error("could not create web push: %v", err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.Status(http.StatusCreated)
}
