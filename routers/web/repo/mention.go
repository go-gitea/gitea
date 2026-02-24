// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/context"
)

type mentionValue struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Name     string `json:"name"`
	FullName string `json:"fullname"`
	Avatar   string `json:"avatar"`
}

// MentionValues returns JSON data for mention autocomplete (assignees, participants, mentionable teams).
func MentionValues(ctx *context.Context) {
	seen := make(map[string]bool)
	var result []mentionValue

	addUser := func(u *user_model.User) {
		if !seen[u.Name] {
			seen[u.Name] = true
			result = append(result, mentionValue{
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
			ctx.ServerError("GetIssueByIndex", err)
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
			result = append(result, mentionValue{
				Key:    key,
				Value:  key,
				Name:   key,
				Avatar: ctx.Repo.Owner.AvatarLink(ctx),
			})
		}
	}

	if result == nil {
		result = []mentionValue{}
	}
	ctx.JSON(http.StatusOK, result)
}
