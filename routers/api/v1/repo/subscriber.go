// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"

	api "code.gitea.io/sdk/gitea"
)

// ListSubscribers list a repo's subscribers (i.e. watchers)
func ListSubscribers(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/subscribers repository repoListSubscribers
	// ---
	// summary: List a repo's watchers
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"
	subscribers, err := ctx.Repo.Repository.GetWatchers(0)
	if err != nil {
		ctx.Error(500, "GetWatchers", err)
		return
	}
	users := make([]*api.User, len(subscribers))
	for i, subscriber := range subscribers {
		users[i] = subscriber.APIFormat()
	}
	ctx.JSON(200, users)
}
