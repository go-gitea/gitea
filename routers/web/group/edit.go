// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"gitea.dev/modules/json"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	group_service "gitea.dev/services/group"
)

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
	ctx.JSONOK()
}
