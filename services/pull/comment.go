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

const maxPushCommitsCount = 1000

func preparePushPullCommentPushActionContent(ctx context.Context, pusher *user_model.User, pr *issues_model.PullRequest, oldCommitID, newCommitID string, isForcePush bool) (data issues_model.PushActionContent, shouldCreate bool, err error) {
	if isForcePush {
		// if it's a force push, we need to get the whole pull request commits
		// the force-push timeline comment should always be created, so all errors are ignored and logged only.
		mergeBase, err := gitrepo.MergeBase(ctx, pr.BaseRepo, pr.BaseBranch, newCommitID)
		if err != nil {
			log.Debug("GetCommitIDsBetween %q..%q failed: %v", mergeBase, newCommitID, err)
		} else {
			data.CommitIDs, err = gitrepo.GetCommitIDsBetweenReversed(ctx, pr.BaseRepo, mergeBase, newCommitID, maxPushCommitsCount)
			if err != nil {
				log.Debug("GetCommitIDsBetween %q..%q failed: %v", mergeBase, newCommitID, err)
			}
		}
		return data, true, nil
	}

	// for a normal push, it maybe an empty pull request, only non-empty pull request need to create push comment
	data.CommitIDs, err = gitrepo.GetCommitIDsBetweenReversed(ctx, pr.BaseRepo, oldCommitID, newCommitID, maxPushCommitsCount)
	return data, len(data.CommitIDs) > 0, err
}

func prepareOldCommitCommentsToDelete(ctx context.Context, oldCommitComments []*issues_model.Comment, data *issues_model.PushActionContent) (needDeleteCommentIDs []int64) {
	commitIDMaps := container.SetOf(data.CommitIDs...)
	for _, oldCommitComment := range oldCommitComments {
		oldData, err := oldCommitComment.GetPushActionContent()
		if err != nil {
			continue
		}
		if oldData.IsForcePush {
			continue
		}

		oldData.CommitIDs = slices.DeleteFunc(oldData.CommitIDs, func(commitID string) bool { return !commitIDMaps.Contains(commitID) })
		if len(oldData.CommitIDs) == 0 {
			needDeleteCommentIDs = append(needDeleteCommentIDs, oldCommitComment.ID)
			continue
		}

		for _, commitID := range oldData.CommitIDs {
			data.CommitIDs = slices.DeleteFunc(data.CommitIDs, func(id string) bool { return id == commitID })
		}
		updatedOldContent, _ := json.Marshal(oldData)
		_, err = db.GetEngine(ctx).ID(oldCommitComment.ID).Cols("content").NoAutoTime().Update(&issues_model.Comment{Content: string(updatedOldContent)})
		if err != nil {
			log.Error("Update Comment content failed: %v", err)
		}
	}
	return needDeleteCommentIDs
}

func cleanUpOldCommitCommentsForNewForcePush(ctx context.Context, pr *issues_model.PullRequest, data *issues_model.PushActionContent) error {
	// All old non-force-push commit comments will be deleted if they are not in the new commit list.
	var oldCommitComments []*issues_model.Comment
	err := db.GetEngine(ctx).Table("comment").
		Where("issue_id = ?", pr.IssueID).And("type = ?", issues_model.CommentTypePullRequestPush).
		Find(&oldCommitComments)
	if err != nil {
		return err
	}

	needDeleteCommentIDs := prepareOldCommitCommentsToDelete(ctx, oldCommitComments, data)
	if len(needDeleteCommentIDs) == 0 {
		return nil
	}
	_, err = db.GetEngine(ctx).In("id", needDeleteCommentIDs).Delete(&issues_model.Comment{})
	return err
}

// CreatePushPullComment create push code to pull base comment
func CreatePushPullComment(ctx context.Context, pusher *user_model.User, pr *issues_model.PullRequest, oldCommitID, newCommitID string, isForcePush bool) (comment *issues_model.Comment, created bool, err error) {
	if pr.HasMerged || oldCommitID == "" || newCommitID == "" {
		return nil, false, nil
	}

	data, shouldCreate, err := preparePushPullCommentPushActionContent(ctx, pusher, pr, oldCommitID, newCommitID, isForcePush)
	if !shouldCreate {
		return nil, false, err
	}

	comment, err = db.WithTx2(ctx, func(ctx context.Context) (comment *issues_model.Comment, err error) {
		if isForcePush {
			err := cleanUpOldCommitCommentsForNewForcePush(ctx, pr, &data)
			if err != nil {
				log.Error("CleanUpOldCommitComments failed: %v", err)
			}
		}

		if len(data.CommitIDs) > 0 {
			dataJSON, _ := json.Marshal(data)
			opts := &issues_model.CreateCommentOptions{
				Type:    issues_model.CommentTypePullRequestPush,
				Doer:    pusher,
				Repo:    pr.BaseRepo,
				Issue:   pr.Issue,
				Content: string(dataJSON),
			}
			comment, err = issues_model.CreateComment(ctx, opts)
			if err != nil {
				return nil, err
			}
		}

		if isForcePush {
			// if it's a force push, we need to add a force push comment
			forcePushDataJSON, _ := json.Marshal(&issues_model.PushActionContent{IsForcePush: true, CommitIDs: []string{oldCommitID, newCommitID}})
			opts := &issues_model.CreateCommentOptions{
				Type:        issues_model.CommentTypePullRequestPush,
				Doer:        pusher,
				Repo:        pr.BaseRepo,
				Issue:       pr.Issue,
				IsForcePush: true,
				Content:     string(forcePushDataJSON), // it seems the field is unnecessary any more because PushActionContent includes IsForcePush field
			}
			comment, err = issues_model.CreateComment(ctx, opts)
			if err != nil {
				return nil, err
			}
		}
		return comment, nil
	})

	if err != nil {
		return nil, false, err
	}
	return comment, true, nil
}
