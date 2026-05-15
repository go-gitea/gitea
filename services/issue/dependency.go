// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"
)

type FilteredDependencies struct {
	Visible []issues_model.DependencyRef
	Hidden  int
}

func GetFilteredDependencyRefs(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) (blockedBy, blocking *FilteredDependencies, err error) {
	repoPerms := make(map[int64]access_model.Permission)
	return getFilteredDependencyRefs(ctx, doer, issue, repoPerms)
}

func GetFilteredDependencyRefsForList(ctx context.Context, doer *user_model.User, issues issues_model.IssueList) (map[int64][2]*FilteredDependencies, error) {
	repoPerms := make(map[int64]access_model.Permission)
	result := make(map[int64][2]*FilteredDependencies, len(issues))

	for _, issue := range issues {
		blockedBy, blocking, err := getFilteredDependencyRefs(ctx, doer, issue, repoPerms)
		if err != nil {
			return nil, err
		}
		result[issue.ID] = [2]*FilteredDependencies{blockedBy, blocking}
	}
	return result, nil
}

func getFilteredDependencyRefs(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, repoPerms map[int64]access_model.Permission) (blockedBy, blocking *FilteredDependencies, err error) {
	blockedByDeps, _, err := issue.BlockedByDependencies(ctx, db.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	blockingDeps, err := issue.BlockingDependencies(ctx)
	if err != nil {
		return nil, nil, err
	}

	blockedBy = filterDependencies(ctx, doer, blockedByDeps, repoPerms)
	blocking = filterDependencies(ctx, doer, blockingDeps, repoPerms)
	return blockedBy, blocking, nil
}

func filterDependencies(ctx context.Context, doer *user_model.User, deps []*issues_model.DependencyInfo, repoPerms map[int64]access_model.Permission) *FilteredDependencies {
	result := &FilteredDependencies{}

	for _, dep := range deps {
		perm, ok := repoPerms[dep.Repository.ID]
		if !ok {
			var err error
			perm, err = access_model.GetDoerRepoPermission(ctx, &dep.Repository, doer)
			if err != nil {
				result.Hidden++
				continue
			}
			repoPerms[dep.Repository.ID] = perm
		}

		if perm.CanReadIssuesOrPulls(dep.Issue.IsPull) {
			result.Visible = append(result.Visible, issues_model.DependencyRef{
				OwnerName: dep.Repository.OwnerName,
				RepoName:  dep.Repository.Name,
				Index:     dep.Issue.Index,
			})
		} else {
			result.Hidden++
		}
	}
	return result
}
