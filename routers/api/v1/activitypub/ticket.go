// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/services/activitypub"
)

// Ticket function returns the Ticket object for an issue or PR
func Ticket(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/ticket/{username}/{reponame}/{id} activitypub forgefedTicket
	// ---
	// summary: Returns the Ticket object for an issue or PR
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: reponame
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: ID number of the issue or PR
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	index, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		ctx.ServerError("ParseInt", err)
		return
	}
	issue, err := issues_model.GetIssueByIndex(ctx.Repo.Repository.ID, index)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}
	ticket, err := activitypub.Ticket(issue)
	if err != nil {
		ctx.ServerError("Ticket", err)
		return
	}
	response(ctx, ticket)
}
