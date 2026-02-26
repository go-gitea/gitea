// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"net/http"

	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/context"
)

type mention struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Name     string `json:"name"`
	FullName string `json:"fullname"`
	Avatar   string `json:"avatar"`
}

// GetMentions returns JSON data for mention autocomplete on org-level pages (members and teams).
func GetMentions(ctx *context.Context) {
	if !ctx.ContextUser.IsOrganization() {
		ctx.JSON(http.StatusOK, []mention{})
		return
	}

	seen := make(map[string]bool)
	var result []mention

	addUser := func(u *user_model.User) {
		if !seen[u.Name] {
			seen[u.Name] = true
			result = append(result, mention{
				Key:      u.Name + " " + u.FullName,
				Value:    u.Name,
				Name:     u.Name,
				FullName: u.FullName,
				Avatar:   u.AvatarLink(ctx),
			})
		}
	}

	org := organization.OrgFromUser(ctx.ContextUser)

	// Get org members
	members, _, err := org.GetMembers(ctx, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetMembers", err)
		return
	}
	for _, u := range members {
		addUser(u)
	}

	// Get mentionable teams
	if ctx.Doer != nil {
		isAdmin := ctx.Doer.IsAdmin
		if !isAdmin {
			isAdmin, err = org.IsOwnedBy(ctx, ctx.Doer.ID)
			if err != nil {
				ctx.ServerError("IsOwnedBy", err)
				return
			}
		}

		var teams []*organization.Team
		if isAdmin {
			teams, err = org.LoadTeams(ctx)
		} else {
			teams, err = org.GetUserTeams(ctx, ctx.Doer.ID)
		}
		if err != nil {
			ctx.ServerError("GetTeams", err)
			return
		}

		for _, team := range teams {
			key := ctx.ContextUser.Name + "/" + team.Name
			if !seen[key] {
				seen[key] = true
				result = append(result, mention{
					Key:    key,
					Value:  key,
					Name:   key,
					Avatar: ctx.ContextUser.AvatarLink(ctx),
				})
			}
		}
	}

	if result == nil {
		result = []mention{}
	}
	ctx.JSON(http.StatusOK, result)
}
