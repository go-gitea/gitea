// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	group_model "code.gitea.io/gitea/models/group"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	group_service "code.gitea.io/gitea/services/group"
)

func MoveGroupItem(ctx *context.Context) {
	form := &forms.MovedGroupItemForm{}
	if err := json.NewDecoder(ctx.Req.Body).Decode(form); err != nil {
		ctx.ServerError("DecodeMovedGroupItemForm", err)
		return
	}
	if form.IsGroup {
		group, err := group_model.GetGroupByID(ctx, form.ItemID)
		if err != nil {
			ctx.ServerError("GetGroupByID", err)
			return
		}
		if group.ParentGroupID != form.NewParent {
			if err = group_model.MoveGroup(ctx, group, form.NewParent, form.NewPos); err != nil {
				ctx.ServerError("MoveGroup", err)
				return
			}
			if err = group_service.RecalculateGroupAccess(ctx, group, false); err != nil {
				ctx.ServerError("RecalculateGroupAccess", err)
			}
		}
	} else {
		repo, err := repo_model.GetRepositoryByID(ctx, form.ItemID)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
		}
		if repo.GroupID != form.NewParent {
			if err = group_service.MoveRepositoryToGroup(ctx, repo, form.NewParent, form.NewPos); err != nil {
				ctx.ServerError("MoveRepositoryToGroup", err)
			}
		}
	}
	ctx.JSONOK()
}
