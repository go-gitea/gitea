// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/forgefed"

	ap "github.com/go-ap/activitypub"
)

func AuthorizeInteraction(ctx *context.Context) {
	uri, err := url.Parse(ctx.Req.URL.Query().Get("uri"))
	if err != nil {
		ctx.ServerError("Could not parse URI", err)
		return
	}
	resp, err := Fetch(uri)
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
		err = FederatedUserNew(ctx, object.(*ap.Person))
		if err != nil {
			ctx.ServerError("FederatedUserNew", err)
			return
		}
		name, err := personIRIToName(object.GetLink())
		if err != nil {
			ctx.ServerError("personIRIToName", err)
			return
		}
		ctx.Redirect(name)
	case forgefed.RepositoryType:
		err = forgefed.OnRepository(object, func(r *forgefed.Repository) error {
			return FederatedRepoNew(ctx, r)
		})
		if err != nil {
			ctx.ServerError("FederatedRepoNew", err)
			return
		}
		username, reponame, err := repositoryIRIToName(object.GetLink())
		if err != nil {
			ctx.ServerError("repositoryIRIToName", err)
			return
		}
		ctx.Redirect(username+"/"+reponame)
	}

	ctx.Status(http.StatusOK)
}
