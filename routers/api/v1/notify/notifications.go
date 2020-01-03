// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package notify

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// NewAvailable check if unread notifications exist
func NewAvailable(ctx *context.APIContext) {
	// swagger:operation GET /notifications/new notification notifyNewAvailable
	// ---
	// summary: Check if unread notifications exist
	// responses:
	//   "204":
	//     description: No unread notification
	//   "302":
	//     description: Unread notification found

	if models.UnreadAvailable(ctx.User) {
		ctx.Status(http.StatusFound)
	} else {
		ctx.Status(http.StatusNoContent)
	}
}
