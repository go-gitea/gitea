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
		switch {
		case ctx.Repo.IsViewBranch:
			ctx.Data["NotFoundPrompt"] = ctx.Locale.Tr("repo.tree_path_not_found_branch", ctx.Repo.TreePath, ctx.Repo.RefName)
			ctx.Data["NotFoundGoBackURL"] = ctx.Repo.RepoLink + "/src/branch/" + url.PathEscape(ctx.Repo.RefName)
		case ctx.Repo.IsViewTag:
			ctx.Data["NotFoundPrompt"] = ctx.Locale.Tr("repo.tree_path_not_found_tag", ctx.Repo.TreePath, ctx.Repo.RefName)
			ctx.Data["NotFoundGoBackURL"] = ctx.Repo.RepoLink + "/src/tag/" + url.PathEscape(ctx.Repo.RefName)
		case ctx.Repo.IsViewCommit:
			ctx.Data["NotFoundPrompt"] = ctx.Locale.Tr("repo.tree_path_not_found_commit", ctx.Repo.TreePath, ctx.Repo.RefName)
			ctx.Data["NotFoundGoBackURL"] = ctx.Repo.RepoLink + "/src/commit/" + url.PathEscape(ctx.Repo.RefName)
		}
		ctx.NotFound(msg, err)
	} else {
		ctx.ServerError(msg, err)
	}
}
