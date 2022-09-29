// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
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

	repo, err := repo_model.GetRepositoryByOwnerAndNameCtx(ctx, ctx.ContextUser.Name, ctx.Repo.Repository.Name)
	if err != nil {
		ctx.ServerError("GetRepositoryByOwnerAndNameCtx", err)
		return
	}
	index, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil {
		ctx.ServerError("ParseInt", err)
		return
	}
	issue, err := issues_model.GetIssueByIndex(repo.ID, index)
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	ticket.Context = ap.IRI(setting.AppURL + ctx.ContextUser.Name + "/" + ctx.Repo.Repository.Name)

	err = issue.LoadPoster()
	if err != nil {
		ctx.ServerError("LoadPoster", err)
		return
	}
	ticket.AttributedTo = ap.IRI(setting.AppURL + "api/v1/activitypub/user/" + issue.Poster.Name)

	ticket.Summary = ap.NaturalLanguageValuesNew()
	err = ticket.Summary.Set("en", ap.Content(issue.Title))
	if err != nil {
		ctx.ServerError("Set Summary", err)
		return
	}

	ticket.Content = ap.NaturalLanguageValuesNew()
	err = ticket.Content.Set("en", ap.Content(issue.Content))
	if err != nil {
		ctx.ServerError("Set Content", err)
		return
	}

	if issue.IsClosed {
		ticket.IsResolved = true
	}

	response(ctx, ticket)
}
