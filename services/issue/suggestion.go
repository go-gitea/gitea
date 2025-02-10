// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"sort"
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
		issues, err = issues_model.FindIssuesSuggestionByKeyword(ctx, repo.ID, keyword, isPull, pageSize)
		if err != nil {
			return nil, err
		}
		indexKeyword, _ := strconv.ParseInt(keyword, 10, 64)
		if indexKeyword > 0 {
			sort.Slice(issues, func(i, j int) bool {
				if issues[i].Index == indexKeyword {
					return true
				}
				if issues[j].Index == indexKeyword {
					return false
				}
				return issues[i].Index > issues[j].Index
			})
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
