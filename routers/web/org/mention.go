// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/modules/util"
	shared_mention "code.gitea.io/gitea/routers/web/shared/mention"
	"code.gitea.io/gitea/services/context"
)

// GetMentionsInOwner returns JSON data for mention autocomplete on owner-level pages.
func GetMentionsInOwner(ctx *context.Context) {
	// for individual users, we don't have a concept of "mentionable" users or teams, so just return an empty list
	if !ctx.ContextUser.IsOrganization() {
		ctx.JSON(http.StatusOK, []shared_mention.Mention{})
		return
	}

	// for org, return members and teams
	c := shared_mention.NewCollector()
	org := organization.OrgFromUser(ctx.ContextUser)

	// Get org members
	members, _, err := org.GetMembers(ctx, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetMembers", err)
		return
	}
	c.AddUsers(ctx, members)

	// Get mentionable teams
	if err := c.AddMentionableTeams(ctx, ctx.Doer, ctx.ContextUser); err != nil {
		ctx.ServerError("AddMentionableTeams", err)
		return
	}

	ctx.JSON(http.StatusOK, util.SliceNilAsEmpty(c.Result))
}
