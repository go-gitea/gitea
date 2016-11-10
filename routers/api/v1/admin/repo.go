// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	api "code.gitea.io/go-sdk/gitea"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/api/v1/repo"
	"code.gitea.io/gitea/routers/api/v1/user"
)

// https://github.com/gogits/go-gogs-client/wiki/Administration-Repositories#create-a-new-repository
func CreateRepo(ctx *context.APIContext, form api.CreateRepoOption) {
	owner := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}

	repo.CreateUserRepo(ctx, owner, form)
}
