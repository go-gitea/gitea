// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/modules/context"
)

// ListStargazers list a repository's stargazers
func ListStargazers(ctx *context.APIContext) {
	stargazers, err := ctx.Repo.Repository.GetStargazers(-1)
	if err != nil {
		ctx.Error(500, "GetStargazers", err)
		return
	}
	users := make([]*api.User, len(stargazers))
	for i, stargazer := range stargazers {
		users[i] = stargazer.APIFormat()
	}
	ctx.JSON(200, users)
}
