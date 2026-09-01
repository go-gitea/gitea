// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"cmp"
	"context"
	"slices"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"

	"xorm.io/builder"
)

// Every issue has its Repo loaded.
type VisibleDependencies struct {
	BlockedBy []*issues_model.Issue
	Blocking  []*issues_model.Issue
}

// LoadVisibleDependencies loads both dependency directions for every issue in the list
// with a fixed number of queries, then drops dependencies that live in repositories the
// doer cannot read issues or pulls in. The result always has an entry for every input issue.
//
// BlockedBy is left empty for issues whose repository has dependencies disabled, matching
// GET /issues/{index}/dependencies. Blocking ignores that setting, matching GET /issues/{index}/blocks:
// another repository may still depend on this issue.
func LoadVisibleDependencies(ctx context.Context, doer *user_model.User, issues issues_model.IssueList) (map[int64]*VisibleDependencies, error) {
	result := make(map[int64]*VisibleDependencies, len(issues))
	inputByID := make(map[int64]*issues_model.Issue, len(issues))
	for _, issue := range issues {
		result[issue.ID] = &VisibleDependencies{}
		inputByID[issue.ID] = issue
	}
	if len(issues) == 0 {
		return result, nil
	}
	if _, err := issues.LoadRepositories(ctx); err != nil {
		return nil, err
	}

	links, err := loadDependencyLinks(ctx, issues)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return result, nil
	}

	linkedByID, err := loadLinkedIssues(ctx, links)
	if err != nil {
		return nil, err
	}

	depsEnabled := make(map[int64]bool, len(issues))
	for _, issue := range issues {
		depsEnabled[issue.ID] = issue.Repo.IsDependenciesEnabled(ctx)
	}

	canRead := newRepoReadChecker(ctx, doer)
	for _, link := range links {
		// link means: issue link.IssueID is blocked by issue link.DependencyID
		if blocked, isInput := inputByID[link.IssueID]; isInput && depsEnabled[blocked.ID] {
			blocker := linkedByID[link.DependencyID]
			visible, err := canRead(blocker)
			if err != nil {
				return nil, err
			}
			if visible {
				result[blocked.ID].BlockedBy = append(result[blocked.ID].BlockedBy, blocker)
			}
		}
		if blocker, isInput := inputByID[link.DependencyID]; isInput {
			blocked := linkedByID[link.IssueID]
			visible, err := canRead(blocked)
			if err != nil {
				return nil, err
			}
			if visible {
				result[blocker.ID].Blocking = append(result[blocker.ID].Blocking, blocked)
			}
		}
	}

	for _, issue := range issues {
		sortLikeDependencyLists(issue, result[issue.ID].BlockedBy)
		sortLikeDependencyLists(issue, result[issue.ID].Blocking)
	}
	return result, nil
}

func loadDependencyLinks(ctx context.Context, issues issues_model.IssueList) ([]*issues_model.IssueDependency, error) {
	issueIDs := make([]int64, 0, len(issues))
	for _, issue := range issues {
		issueIDs = append(issueIDs, issue.ID)
	}
	var links []*issues_model.IssueDependency
	err := db.GetEngine(ctx).
		Where(builder.Or(builder.In("issue_id", issueIDs), builder.In("dependency_id", issueIDs))).
		Find(&links)
	return links, err
}

func loadLinkedIssues(ctx context.Context, links []*issues_model.IssueDependency) (map[int64]*issues_model.Issue, error) {
	ids := make(container.Set[int64], len(links)*2)
	for _, link := range links {
		ids.Add(link.IssueID)
		ids.Add(link.DependencyID)
	}
	linked, err := issues_model.GetIssuesByIDs(ctx, ids.Values())
	if err != nil {
		return nil, err
	}
	if _, err := linked.LoadRepositories(ctx); err != nil {
		return nil, err
	}
	byID := make(map[int64]*issues_model.Issue, len(linked))
	for _, issue := range linked {
		byID[issue.ID] = issue
	}
	return byID, nil
}

// A nil issue (dangling link) is never readable.
func newRepoReadChecker(ctx context.Context, doer *user_model.User) func(*issues_model.Issue) (bool, error) {
	perms := make(map[int64]access_model.Permission)
	return func(issue *issues_model.Issue) (bool, error) {
		if issue == nil || issue.Repo == nil {
			return false, nil
		}
		perm, ok := perms[issue.RepoID]
		if !ok {
			var err error
			perm, err = access_model.GetDoerRepoPermission(ctx, issue.Repo, doer)
			if err != nil {
				return false, err
			}
			perms[issue.RepoID] = perm
		}
		return perm.CanReadIssuesOrPulls(issue.IsPull), nil
	}
}

// sortLikeDependencyLists orders like Issue.BlockedByDependencies: same repository first,
// other repositories by id, newest first within a repository.
func sortLikeDependencyLists(of *issues_model.Issue, deps []*issues_model.Issue) {
	slices.SortStableFunc(deps, func(a, b *issues_model.Issue) int {
		aSame, bSame := a.RepoID == of.RepoID, b.RepoID == of.RepoID
		if aSame != bSame {
			if aSame {
				return -1
			}
			return 1
		}
		if a.RepoID != b.RepoID {
			return cmp.Compare(a.RepoID, b.RepoID)
		}
		if a.CreatedUnix != b.CreatedUnix {
			return cmp.Compare(b.CreatedUnix, a.CreatedUnix)
		}
		return cmp.Compare(b.ID, a.ID)
	})
}
