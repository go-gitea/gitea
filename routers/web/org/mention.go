// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/models/organization"
	shared_mention "code.gitea.io/gitea/routers/web/shared/mention"
	"code.gitea.io/gitea/services/context"
)

// GetMentions returns JSON data for mention autocomplete on org-level pages (members and teams).
func GetMentions(ctx *context.Context) {
	if !ctx.ContextUser.IsOrganization() {
		ctx.JSON(http.StatusOK, []shared_mention.Mention{})
		return
	}

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

	ctx.JSON(http.StatusOK, c.ResultOrEmpty())
}
