// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/structs"
)

func GetSuggestion(ctx context.Context, repo *repo_model.Repository, isPull optional.Option[bool], keyword string) ([]*structs.Issue, error) {
	var issues issues_model.IssueList
	var err error
	pageSize := 5
	if keyword == "" {
		issues, err = issues_model.FindLatestIssues(ctx, repo.ID, isPull, pageSize)
		if err != nil {
			return nil, err
		}
	} else {
		indexKeyword, _ := strconv.ParseInt(keyword, 10, 64)
		nonIDs := []int64{}
		if indexKeyword > 0 {
			issues, err = issues_model.FindIssuesWithIndexPrefix(ctx, repo.ID, indexKeyword, isPull, pageSize)
			if err != nil {
				return nil, err
			}
			for _, issue := range issues {
				nonIDs = append(nonIDs, issue.ID)
			}
			pageSize -= len(issues)
		}

		if pageSize > 0 {
			newIssues, err := issues_model.FindIssuesTitleKeywords(ctx, repo.ID, keyword, isPull, nonIDs, pageSize)
			if err != nil {
				return nil, err
			}
			issues = append(issues, newIssues...)
		}
	}

	if err := issues.LoadPullRequests(ctx); err != nil {
		return nil, err
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

	return suggestions, nil
}
