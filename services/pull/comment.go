// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
)

// getCommitIDsFromRepo get commit IDs from repo in between oldCommitID and newCommitID
// Commit on baseBranch will skip
func getCommitIDsFromRepo(ctx context.Context, repo *repo_model.Repository, oldCommitID, newCommitID, baseBranch string) (commitIDs []string, err error) {
	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	oldCommit, err := gitRepo.GetCommit(oldCommitID)
	if err != nil {
		return nil, err
	}

	newCommit, err := gitRepo.GetCommit(newCommitID)
	if err != nil {
		return nil, err
	}

	// Find commits between new and old commit excluding base branch commits
	commits, err := gitRepo.CommitsBetweenNotBase(newCommit, oldCommit, baseBranch)
	if err != nil {
		return nil, err
	}

	commitIDs = make([]string, 0, len(commits))
	for i := len(commits) - 1; i >= 0; i-- {
		commitIDs = append(commitIDs, commits[i].ID.String())
	}

	return commitIDs, err
}

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
	data.CommitIDs, err = getCommitIDsFromRepo(ctx, pr.BaseRepo, oldCommitID, newCommitID, pr.BaseBranch)
	if err != nil {
		// For force-push events, a missing/unreachable old commit should not prevent
		// deleting stale push comments or creating the force-push timeline entry.
		if !isForcePush {
			return nil, err
		}
		log.Error("getCommitIDsFromRepo: %v", err)
	}
	// It maybe an empty pull request. Only non-empty pull request need to create push comment
	// for force push, we always need to delete the old push comment so don't return here.
	if len(data.CommitIDs) == 0 && !isForcePush {
		return nil, nil //nolint:nilnil // return nil because no comment needs to be created
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
