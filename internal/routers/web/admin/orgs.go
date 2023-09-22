// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package admin

import (
	"code.gitea.io/gitea/internal/models/db"
	user_model "code.gitea.io/gitea/internal/models/user"
	"code.gitea.io/gitea/internal/modules/base"
	"code.gitea.io/gitea/internal/modules/context"
	"code.gitea.io/gitea/internal/modules/setting"
	"code.gitea.io/gitea/internal/modules/structs"
	"code.gitea.io/gitea/internal/routers/web/explore"
)

const (
	tplOrgs base.TplName = "admin/org/list"
)

// Organizations show all the organizations
func Organizations(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.organizations")
	ctx.Data["PageIsAdminOrganizations"] = true

	if ctx.FormString("sort") == "" {
		ctx.SetFormString("sort", explore.UserSearchDefaultAdminSort)
	}

	explore.RenderUserSearch(ctx, &user_model.SearchUserOptions{
		Actor:           ctx.Doer,
		Type:            user_model.UserTypeOrganization,
		IncludeReserved: true, // administrator needs to list all acounts include reserved
		ListOptions: db.ListOptions{
			PageSize: setting.UI.Admin.OrgPagingNum,
		},
		Visible: []structs.VisibleType{structs.VisibleTypePublic, structs.VisibleTypeLimited, structs.VisibleTypePrivate},
	}, tplOrgs)
}
