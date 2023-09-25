// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/url"
	"sort"

	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
)

func MakeSelfOnTop(doer *user.User, users []*user.User) []*user.User {
	if doer != nil {
		sort.Slice(users, func(i, j int) bool {
			if users[i].ID == users[j].ID {
				return false
			}
			return users[i].ID == doer.ID // if users[i] is self, put it before others, so less=true
		})
	}
	return users
}

func HandleGitError(ctx *context.Context, msg string, err error) {
	if git.IsErrNotExist(err) {
		ctx.Data["NotFoundPrompt"] = ctx.Locale.Tr("repo.tree_path_not_found", ctx.Repo.TreePath, ctx.Repo.BranchName)
		ctx.Data["NotFoundGoBackURL"] = ctx.Repo.RepoLink + "/src/branch/" + url.PathEscape(ctx.Repo.BranchName)
		ctx.NotFound(msg, err)
	} else {
		ctx.ServerError(msg, err)
	}
}
