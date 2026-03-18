// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
)

// CreatePushPullComment create push code to pull base comment
func CreatePushPullComment(ctx context.Context, pusher *user_model.User, pr *issues_model.PullRequest, oldCommitID, newCommitID string, isForcePush bool) (comment *issues_model.Comment, err error) {
	if pr.HasMerged || oldCommitID == "" || newCommitID == "" {
		return nil, nil //nolint:nilnil // return nil because no comment needs to be created
	}

	opts := &issues_model.CreateCommentOptions{
		Type:  issues_model.CommentTypePullRequestPush,
		Doer:  pusher,
		Repo:  pr.BaseRepo,
		Issue: pr.Issue,
	}

	var data issues_model.PushActionContent
	if isForcePush {
		// if it's a force push, we need to get the whole pull request commits
		data.CommitIDs, err = gitrepo.GetCommitIDsBetween(ctx, pr.BaseRepo, pr.BaseBranch, newCommitID, true)
		if err != nil {
			// For force-push events, failures resolving the base ref/head commit or computing
			// the merge-base should not prevent deleting stale push comments or creating the
			// force-push timeline entry.
			log.Error("GetCompareCommitIDsWithMergeBase: %v", err)
		}
	} else {
		data.CommitIDs, err = gitrepo.GetCommitIDsBetween(ctx, pr.BaseRepo, oldCommitID, newCommitID, false)
		if err != nil {
			return nil, err
		}
		// It maybe an empty pull request. Only non-empty pull request need to create push comment
		// for force push, we always need to delete the old push comment so don't return here.
		if len(data.CommitIDs) == 0 {
			return nil, nil //nolint:nilnil // return nil because no comment needs to be created
		}
	}

	return db.WithTx2(ctx, func(ctx context.Context) (*issues_model.Comment, error) {
		if isForcePush {
			// Push commits comment should not have history, cross references, reactions and other
			// plain comment related records, so that we just need to delete the comment itself.
			if _, err := db.GetEngine(ctx).Where("issue_id = ?", pr.IssueID).
				And("type = ?", issues_model.CommentTypePullRequestPush).
				NoAutoCondition().
				Delete(new(issues_model.Comment)); err != nil {
				return nil, err
			}
		}

		if len(data.CommitIDs) > 0 {
			dataJSON, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}
			opts.Content = string(dataJSON)
			comment, err = issues_model.CreateComment(ctx, opts)
			if err != nil {
				return nil, err
			}
		}

		if isForcePush { // if it's a force push, we need to add a force push comment
			data.CommitIDs = []string{oldCommitID, newCommitID}
			data.IsForcePush = true
			dataJSON, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}
			opts.Content = string(dataJSON)
			opts.IsForcePush = true // FIXME: it seems the field is unnecessary any more because PushActionContent includes IsForcePush field
			comment, err = issues_model.CreateComment(ctx, opts)
			if err != nil {
				return nil, err
			}
		}

		return comment, err
	})
}
