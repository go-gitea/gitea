// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/models/forgefed"
	"code.gitea.io/gitea/modules/context"

	ap "github.com/go-ap/activitypub"
)

func AuthorizeInteraction(c *context.Context) {
	uri, err := url.Parse(c.Req.URL.Query().Get("uri"))
	if err != nil {
		c.ServerError("Could not parse URI", err)
		return
	}
	resp, err := Fetch(uri)
	if err != nil {
		c.ServerError("Fetch", err)
		return
	}

	ap.ItemTyperFunc = forgefed.GetItemByType
	object, err := ap.UnmarshalJSON(resp)
	if err != nil {
		c.ServerError("UnmarshalJSON", err)
		return
	}

	switch object.GetType() {
	case ap.PersonType:
		if err != nil {
			c.ServerError("UnmarshalJSON", err)
			return
		}
		err = FederatedUserNew(c, object.(ap.Person))
		if err != nil {
			c.ServerError("FederatedUserNew", err)
			return
		}
		name, err := personIRIToName(object.GetLink())
		if err != nil {
			c.ServerError("personIRIToName", err)
			return
		}
		c.Redirect(name)
	case forgefed.RepositoryType:
		err = FederatedRepoNew(object.(forgefed.Repository))
	}

	c.Status(http.StatusOK)
}
