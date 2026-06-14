// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"net/http"

	group_model "gitea.dev/models/group"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/json"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	group_service "gitea.dev/services/group"
)

type movedSubItem struct {
	ID       int64  `json:"id"`
	IsGroup  bool   `json:"isGroup"`
	NewPath  string `json:"newPath"`
	FullName string `json:"fullName,omitempty"`
}
type moveResult struct {
	NewPath  string         `json:"newPath"`
	FullName string         `json:"fullName,omitempty"`
}

func MoveGroupItem(ctx *context.Context) {
	form := &forms.MovedGroupItemForm{}
	if err := json.NewDecoder(ctx.Req.Body).Decode(form); err != nil {
		ctx.ServerError("DecodeMovedGroupItemForm", err)
		return
	}
	if err := group_service.MoveGroupItem(ctx, group_service.MoveGroupOptions{
		IsGroup:   form.IsGroup,
		ItemID:    form.ItemID,
		NewPos:    form.NewPos,
		NewParent: form.NewParent,
	}, ctx.Doer); err != nil {
		ctx.ServerError("MoveGroupItem", err)
		return
	}
	var newPath, fullName string
	if form.IsGroup {
		grp, err := group_model.GetGroupByID(ctx, form.ItemID)
		if err != nil {
			ctx.ServerError("GetGroupByID", err)
			return
		}
		newPath = grp.GroupLink()
	} else {
		repo, err := repo_model.GetRepositoryByID(ctx, form.ItemID)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
			return
		}
		fullName = repo.FullName()
		newPath = repo.Link()
	}
	ctx.JSON(http.StatusOK, moveResult{
		NewPath:  newPath,
		FullName: fullName,
	})
}
