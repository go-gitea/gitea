// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"
	"net/url"
	"strconv"

	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/forgefed"
	user_service "code.gitea.io/gitea/services/user"

	ap "github.com/go-ap/activitypub"
)

func AuthorizeInteraction(ctx *context.Context) {
	uri, err := url.Parse(ctx.Req.URL.Query().Get("uri"))
	if err != nil {
		ctx.ServerError("Parse URI", err)
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
		// Federated user
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
			ctx.ServerError("PersonIRIToName", err)
			return
		}
		ctx.Redirect(name)
	case forgefed.RepositoryType:
		// Federated repository
		err = forgefed.OnRepository(object, func(r *forgefed.Repository) error {
			ownerURL, err := url.Parse(r.AttributedTo.GetLink().String())
			if err != nil {
				return err
			}
			// Fetch person object
			resp, err := activitypub.Fetch(ownerURL)
			if err != nil {
				return err
			}
			// Parse person object
			ap.ItemTyperFunc = forgefed.GetItemByType
			ap.JSONItemUnmarshal = forgefed.JSONUnmarshalerFn
			object, err := ap.UnmarshalJSON(resp)
			if err != nil {
				return err
			}
			// Create federated user
			err = user_service.FederatedUserNew(ctx, object.(*ap.Person))
			if err != nil {
				return err
			}
			return activitypub.FederatedRepoNew(ctx, r)
		})
		if err != nil {
			ctx.ServerError("FederatedRepoNew", err)
			return
		}
		username, reponame, err := activitypub.RepositoryIRIToName(object.GetLink())
		if err != nil {
			ctx.ServerError("RepositoryIRIToName", err)
			return
		}
		ctx.Redirect(username + "/" + reponame)
	case forgefed.TicketType:
		// Federated ticket
		err = forgefed.OnTicket(object, func(t *forgefed.Ticket) error {
			// TODO: make sure federated user exists
			// Also, refactor this code to reduce the chance of accidentally creating import cycles
			repoURL, err := url.Parse(t.Context.GetLink().String())
			if err != nil {
				return err
			}
			// Fetch repository object
			resp, err := activitypub.Fetch(repoURL)
			if err != nil {
				return err
			}
			// Parse repository object
			ap.ItemTyperFunc = forgefed.GetItemByType
			ap.JSONItemUnmarshal = forgefed.JSONUnmarshalerFn
			object, err := ap.UnmarshalJSON(resp)
			if err != nil {
				return err
			}
			// Create federated repo
			err = forgefed.OnRepository(object, func(r *forgefed.Repository) error {
				return activitypub.FederatedRepoNew(ctx, r)
			})
			if err != nil {
				return err
			}
			return activitypub.ReceiveIssue(ctx, t)
		})
		if err != nil {
			ctx.ServerError("ReceiveIssue", err)
			return
		}
		username, reponame, idx, err := activitypub.TicketIRIToName(object.GetLink())
		if err != nil {
			ctx.ServerError("TicketIRIToName", err)
			return
		}
		ctx.Redirect(username + "/" + reponame + "/issues/" + strconv.FormatInt(idx, 10))
	default:
		ctx.ServerError("Not implemented", err)
		return
	}

	ctx.Status(http.StatusOK)
}
