// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"strconv"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
)

func FindIssueDevLinksByIssue(ctx context.Context, issue *issues_model.Issue) (issues_model.IssueDevLinks, error) {
	devLinks, err := issues_model.FindIssueDevLinksByIssueID(ctx, issue.ID)
	if err != nil {
		return nil, err
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}

	for _, link := range devLinks {
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
			pullID, err := strconv.ParseInt(link.LinkIndex, 10, 64)
			if err != nil {
				return nil, err
			}
			pull, err := issues_model.GetPullRequestByID(ctx, pullID)
			if err != nil {
				return nil, err
			}
			link.PullRequest = pull
			link.PullRequest.Issue = issue
			link.PullRequest.BaseRepo = issue.Repo
		case issues_model.IssueDevLinkTypeBranch:
			branch, err := git_model.GetBranch(ctx, link.LinkedRepoID, link.LinkIndex)
			if err != nil {
				return nil, err
			}
			link.Branch = branch
			link.Branch.Repo = link.LinkedRepo
		}
	}

	return devLinks, nil
}
