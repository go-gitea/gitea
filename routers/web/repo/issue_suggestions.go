// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"sort"
	"strconv"

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

	indexKeyword, _ := strconv.ParseInt(keyword, 10, 64)
	pageSize := 5
	issues := make(issues_model.IssueList, 0, pageSize)
	if indexKeyword > 0 {
		issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, indexKeyword)
		if err != nil && !issues_model.IsErrIssueNotExist(err) {
			ctx.ServerError("GetIssueByIndex", err)
			return
		}
		if issue != nil {
			pageSize--
		}

		issues, err = issues_model.FindIssuesWithIndexPrefix(ctx, ctx.Repo.Repository.ID, indexKeyword, pageSize)
		if err != nil {
			ctx.ServerError("FindIssuesWithIndexPrefix", err)
			return
		}
		pageSize -= len(issues)
		if issue != nil {
			issues = append([]*issues_model.Issue{issue}, issues...)
		}
	}

	if pageSize > 0 {
		searchOpt := &issue_indexer.SearchOptions{
			Paginator: &db.ListOptions{
				Page:     0,
				PageSize: pageSize,
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

		for i := 0; i < len(ids); i++ {
			for _, issue := range issues {
				if ids[i] == issue.ID {
					ids = append(ids[:i], ids[i+1:]...)
					i--
					break
				}
			}
		}

		if len(ids) > 0 {
			newIssues, err := issues_model.GetIssuesByIDs(ctx, ids, true)
			if err != nil {
				ctx.ServerError("FindIssuesByIDs", err)
				return
			}
			sort.Slice(newIssues, func(i, j int) bool {
				return newIssues[i].Index > newIssues[j].Index
			})
			issues = append(issues, newIssues...)
		}
	}

	if err := issues.LoadPullRequests(ctx); err != nil {
		ctx.ServerError("LoadPullRequests", err)
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

		if issue.IsPull && issue.PullRequest != nil {
			suggestion.PullRequest = &structs.PullRequestMeta{
				HasMerged:        issue.PullRequest.HasMerged,
				IsWorkInProgress: issue.PullRequest.IsWorkInProgress(ctx),
			}
		}

		suggestions = append(suggestions, suggestion)
	}

	ctx.JSON(http.StatusOK, suggestions)
}
