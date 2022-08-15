// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
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

	link := setting.AppURL + "api/v1/activitypub/ticket/" + ctx.ContextUser.Name + "/" + ctx.Repo.Repository.Name + "/" + ctx.Params("id")

	ticket := forgefed.TicketNew()
	ticket.ID = ap.IRI(link)

	// TODO: Add other ticket fields according to https://forgefed.org/modeling.html#ticket

	response(ctx, ticket)
}
