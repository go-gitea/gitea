// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/forgefed"
	user_service "code.gitea.io/gitea/services/user"

	ap "github.com/go-ap/activitypub"
)

func AuthorizeInteraction(ctx *context.Context) {
	uri, err := url.Parse(ctx.Req.URL.Query().Get("uri"))
	if err != nil {
		ctx.ServerError("Could not parse URI", err)
		return
	}
	resp, err := activitypub.Fetch(uri)
	if err != nil {
		ctx.ServerError("Fetch", err)
		return
	}

	ap.ItemTyperFunc = forgefed.GetItemByType
	ap.JSONItemUnmarshal = forgefed.JSONUnmarshalerFn
	object, err := ap.UnmarshalJSON(resp)
	if err != nil {
		ctx.ServerError("UnmarshalJSON", err)
		return
	}

	switch object.GetType() {
	case ap.PersonType:
		if err != nil {
			ctx.ServerError("UnmarshalJSON", err)
			return
		}
		err = user_service.FederatedUserNew(ctx, object.(*ap.Person))
		if err != nil {
			ctx.ServerError("FederatedUserNew", err)
			return
		}
		name, err := activitypub.PersonIRIToName(object.GetLink())
		if err != nil {
			ctx.ServerError("personIRIToName", err)
			return
		}
		ctx.Redirect(name)
	case forgefed.RepositoryType:
		err = forgefed.OnRepository(object, func(r *forgefed.Repository) error {
			return activitypub.FederatedRepoNew(ctx, r)
		})
		if err != nil {
			ctx.ServerError("FederatedRepoNew", err)
			return
		}
		username, reponame, err := activitypub.RepositoryIRIToName(object.GetLink())
		if err != nil {
			ctx.ServerError("repositoryIRIToName", err)
			return
		}
		ctx.Redirect(username + "/" + reponame)
	case forgefed.TicketType:
		err = forgefed.OnTicket(object, func(t *forgefed.Ticket) error {
			return activitypub.ReceiveIssue(ctx, t)
		})
		if err != nil {
			ctx.ServerError("ReceiveIssue", err)
			return
		}
		// TODO: Implement ticketIRIToName and redirect to ticket
	}

	ctx.Status(http.StatusOK)
}
