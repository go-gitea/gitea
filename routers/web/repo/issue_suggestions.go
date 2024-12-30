// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unit"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
)

// IssueSuggestions returns a list of issue suggestions
func IssueSuggestions(ctx *context.Context) {
	keyword := ctx.Req.FormValue("q")

	canReadIssues := ctx.Repo.CanRead(unit.TypeIssues)
	canReadPulls := ctx.Repo.CanRead(unit.TypePullRequests)

	var isPull optional.Option[bool]
	if canReadPulls && !canReadIssues {
		isPull = optional.Some(true)
	} else if canReadIssues && !canReadPulls {
		isPull = optional.Some(false)
	}

	searchOpt := &issue_indexer.SearchOptions{
		Paginator: &db.ListOptions{
			Page:     0,
			PageSize: 5,
		},
		Keyword:  keyword,
		RepoIDs:  []int64{ctx.Repo.Repository.ID},
		IsPull:   isPull,
		IsClosed: nil,
		SortBy:   issue_indexer.SortByUpdatedDesc,
	}

	ids, _, err := issue_indexer.SearchIssues(ctx, searchOpt)
	if err != nil {
		ctx.ServerError("SearchIssues", err)
		return
	}
	issues, err := issues_model.GetIssuesByIDs(ctx, ids, true)
	if err != nil {
		ctx.ServerError("FindIssuesByIDs", err)
		return
	}

	suggestions := make([]*structs.Issue, 0, len(issues))

	for _, issue := range issues {
		suggestion := &structs.Issue{
			ID:    issue.ID,
			Index: issue.Index,
			Title: issue.Title,
			State: issue.State(),
		}

		if issue.IsPull {
			if err := issue.LoadPullRequest(ctx); err != nil {
				ctx.ServerError("LoadPullRequest", err)
				return
			}
			if issue.PullRequest != nil {
				suggestion.PullRequest = &structs.PullRequestMeta{
					HasMerged:        issue.PullRequest.HasMerged,
					IsWorkInProgress: issue.PullRequest.IsWorkInProgress(ctx),
				}
			}
		}

		suggestions = append(suggestions, suggestion)
	}

	ctx.JSON(http.StatusOK, suggestions)
}
