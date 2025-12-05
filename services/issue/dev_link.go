// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"
	"sort"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/container"
)

func FindIssueDevLinksByIssue(ctx context.Context, issue *issues_model.Issue) (issues_model.IssueDevLinks, error) {
	devLinks, err := issues_model.FindIssueDevLinksByIssueID(ctx, issue.ID)
	if err != nil {
		return nil, err
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}

	sort.Slice(devLinks, func(i, j int) bool {
		return devLinks[j].LinkType != issues_model.IssueDevLinkTypePullRequest
	})

	branchPRExists := make(container.Set[string])

	for _, link := range devLinks {
		link.Repo = issue.Repo
		if link.LinkedRepoID == 0 {
			link.LinkedRepoID = issue.RepoID
		}
		isSameRepo := issue.RepoID == link.LinkedRepoID
		if isSameRepo {
			link.LinkedRepo = issue.Repo
		} else if link.LinkedRepoID > 0 {
			repo, err := repo_model.GetRepositoryByID(ctx, link.LinkedRepoID)
			if err != nil {
				return nil, err
			}
			link.LinkedRepo = repo
		}

		switch link.LinkType {
		case issues_model.IssueDevLinkTypePullRequest:
			pull, err := issues_model.GetPullRequestByID(ctx, link.LinkID)
			if err != nil {
				return nil, err
			}
			pull.BaseRepo = issue.Repo
			pull.HeadRepo = link.LinkedRepo
			if err := pull.LoadIssue(ctx); err != nil {
				return nil, err
			}
			pull.Issue.Repo = issue.Repo
			link.PullRequest = pull
			branchPRExists.Add(fmt.Sprintf("%d-%d-%s", link.LinkedRepoID, link.LinkType, pull.HeadBranch))
		case issues_model.IssueDevLinkTypeBranch:
			branch, err := git_model.GetBranchByID(ctx, link.LinkID)
			if err != nil {
				return nil, err
			}
			link.Branch = branch
			link.Branch.Repo = link.LinkedRepo
			link.DisplayBranch = !branchPRExists.Contains(fmt.Sprintf("%d-%d-%d", link.LinkedRepoID, link.LinkType, link.LinkID))
		}
	}

	return devLinks, nil
}
