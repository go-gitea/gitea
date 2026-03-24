// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"slices"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
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
	commitIDMaps := make(container.Set[string])
	if isForcePush {
		// if it's a force push, we need to get the whole pull request commits
		mergeBase, err := gitrepo.MergeBase(ctx, pr.BaseRepo, pr.BaseBranch, newCommitID)
		if err != nil {
			// For force-push events, failures resolving the base ref/head commit or computing the merge-base should not prevent deleting stale push comments or creating the force-push timeline entry.
			log.Error("MergeBase %q and %q failed: %v", pr.BaseBranch, newCommitID, err)
		} else {
			data.CommitIDs, err = gitrepo.GetCommitIDsBetweenReversed(ctx, pr.BaseRepo, mergeBase, newCommitID)
			if err != nil {
				// For force-push events, failures resolving the base ref/head commit or computing
				// the merge-base should not prevent deleting stale push comments or creating the
				// force-push timeline entry.
				log.Error("GetCommitIDsBetween %q..%q failed: %v", mergeBase, newCommitID, err)
			}
			for _, commitID := range data.CommitIDs {
				commitIDMaps.Add(commitID)
			}
		}
	} else {
		data.CommitIDs, err = gitrepo.GetCommitIDsBetweenReversed(ctx, pr.BaseRepo, oldCommitID, newCommitID)
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
			// All old non-force-push commit comments will be deleted if they are not in the new commit list.
			var oldCommitComments []*issues_model.Comment
			if err := db.GetEngine(ctx).
				Table("comment").
				Where("issue_id = ?", pr.IssueID).
				And("type = ?", issues_model.CommentTypePullRequestPush).
				NoAutoCondition().
				Find(&oldCommitComments); err != nil {
				return nil, err
			}
			var needDeleteCommentIDs []int64
			for _, oldCommitComment := range oldCommitComments {
				a, err := oldCommitComment.GetPushActionContent()
				if err != nil {
					log.Error("GetPushActionContent failed: %v", err)
					continue
				}
				if !a.IsForcePush {
					a.CommitIDs = slices.DeleteFunc(a.CommitIDs, func(commitID string) bool {
						return !commitIDMaps.Contains(commitID)
					})
					if len(a.CommitIDs) == 0 {
						needDeleteCommentIDs = append(needDeleteCommentIDs, oldCommitComment.ID)
					} else {
						for _, commitID := range a.CommitIDs {
							data.CommitIDs = slices.DeleteFunc(data.CommitIDs, func(id string) bool {
								return id == commitID
							})
						}
						dataJSON, err := json.Marshal(a)
						if err != nil {
							log.Error("Marshal PushActionContent failed: %v", err)
							continue
						}
						if _, err := db.GetEngine(ctx).
							ID(oldCommitComment.ID).
							Cols("content").
							NoAutoTime().
							Update(&issues_model.Comment{Content: string(dataJSON)}); err != nil {
							log.Error("Update Comment content failed: %v", err)
							continue
						}
					}
				}
			}
			if len(needDeleteCommentIDs) > 0 {
				if _, err := db.GetEngine(ctx).
					In("id", needDeleteCommentIDs).
					Delete(&issues_model.Comment{}); err != nil {
					return nil, err
				}
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
