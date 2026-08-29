// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"errors"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/util"
)

func EnumPullRequestsByHeadCommitID(ctx context.Context, repo *repo_model.Repository, commitID string, filter func(*issues_model.PullRequest) bool) (pulls []*issues_model.PullRequest, _ error) {
	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	refs, err := gitRepo.GetRefsBySha(ctx, commitID, git.PullPrefix)
	if err != nil {
		return nil, err
	}

	for _, refStr := range refs {
		ref := git.RefName(refStr)
		prIndex, ok := ref.PullIndex()
		if !ok {
			continue
		}

		pull, err := issues_model.GetPullRequestByIndex(ctx, repo.ID, prIndex)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				continue // ignore non-existing pull requests
			}
			return nil, err
		}

		if filter(pull) {
			pulls = append(pulls, pull)
		}
	}
	return pulls, nil
}

func GetMergeablePullRequestsByHeadCommitID(ctx context.Context, repo *repo_model.Repository, commitID string) ([]*issues_model.PullRequest, error) {
	return EnumPullRequestsByHeadCommitID(ctx, repo, commitID, func(pr *issues_model.PullRequest) bool {
		_ = pr.LoadIssue(ctx)
		return pr.Issue != nil && !pr.Issue.IsClosed && !pr.HasMerged && pr.IsStatusMergeable()
	})
}
