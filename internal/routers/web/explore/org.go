// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"code.gitea.io/gitea/internal/models/db"
	user_model "code.gitea.io/gitea/internal/models/user"
	"code.gitea.io/gitea/internal/modules/context"
	"code.gitea.io/gitea/internal/modules/setting"
	"code.gitea.io/gitea/internal/modules/structs"
)

// Organizations render explore organizations page
func Organizations(ctx *context.Context) {
	ctx.Data["UsersIsDisabled"] = setting.Service.Explore.DisableUsersPage
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExploreOrganizations"] = true
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	visibleTypes := []structs.VisibleType{structs.VisibleTypePublic}
	if ctx.Doer != nil {
		visibleTypes = append(visibleTypes, structs.VisibleTypeLimited, structs.VisibleTypePrivate)
	}

	if ctx.FormString("sort") == "" {
		ctx.SetFormString("sort", UserSearchDefaultSortType)
	}

	RenderUserSearch(ctx, &user_model.SearchUserOptions{
		Actor:       ctx.Doer,
		Type:        user_model.UserTypeOrganization,
		ListOptions: db.ListOptions{PageSize: setting.UI.ExplorePagingNum},
		Visible:     visibleTypes,
	}, tplExploreUsers)
}
