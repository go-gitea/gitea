// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
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

// GetMentions returns JSON data for mention autocomplete (assignees, participants, mentionable teams).
func GetMentions(ctx *context.Context) {
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

	// Get participants if issue_index is provided
	if issueIndex := ctx.FormInt64("issue_index"); issueIndex > 0 {
		issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, issueIndex)
		if err != nil {
			ctx.NotFoundOrServerError("GetIssueByIndex", issues_model.IsErrIssueNotExist, err)
			return
		}
		userIDs, err := issue.GetParticipantIDsByIssue(ctx)
		if err != nil {
			ctx.ServerError("GetParticipantIDsByIssue", err)
			return
		}
		if len(userIDs) > 0 {
			users, err := user_model.GetUsersByIDs(ctx, userIDs)
			if err != nil {
				ctx.ServerError("GetUsersByIDs", err)
				return
			}
			for _, u := range users {
				addUser(u)
			}
		}
	}

	// Get repo assignees
	assignees, err := repo_model.GetRepoAssignees(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	for _, u := range assignees {
		addUser(u)
	}

	// Get mentionable teams for org repos
	teams, err := getMentionableTeams(ctx)
	if err != nil {
		ctx.ServerError("getMentionableTeams", err)
		return
	}
	for _, team := range teams {
		key := ctx.Repo.Owner.Name + "/" + team.Name
		if !seen[key] {
			seen[key] = true
			result = append(result, mention{
				Key:    key,
				Value:  key,
				Name:   key,
				Avatar: ctx.Repo.Owner.AvatarLink(ctx),
			})
		}
	}

	if result == nil {
		result = []mention{}
	}
	ctx.JSON(http.StatusOK, result)
}

// getMentionableTeams returns the teams that the current user can mention in the repo context.
func getMentionableTeams(ctx *context.Context) ([]*organization.Team, error) {
	if ctx.Doer == nil || !ctx.Repo.Owner.IsOrganization() {
		return nil, nil
	}

	org := organization.OrgFromUser(ctx.Repo.Owner)
	// Admin has super access.
	isAdmin := ctx.Doer.IsAdmin
	if !isAdmin {
		var err error
		isAdmin, err = org.IsOwnedBy(ctx, ctx.Doer.ID)
		if err != nil {
			return nil, err
		}
	}

	if isAdmin {
		return org.LoadTeams(ctx)
	}
	return org.GetUserTeams(ctx, ctx.Doer.ID)
}
