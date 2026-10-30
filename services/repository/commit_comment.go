// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"slices"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/references"
	notify_service "gitea.dev/services/notify"
)

func CreateCommitComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, commit *git.Commit, treePath string, line int64, content string) error {
	blockerIDs := []int64{repo.OwnerID}
	if author := user_model.GetUserByGitAuthor(ctx, commit); author != nil {
		blockerIDs = append(blockerIDs, author.ID)
	}
	if user_model.IsUserBlockedBy(ctx, doer, blockerIDs...) {
		return user_model.ErrBlockedUser
	}

	comment := &issues_model.Comment{
		PosterID: doer.ID,
		Poster:   doer,
		TreePath: treePath,
		Line:     line,
		Content:  content,
	}
	if err := issues_model.CreateCommitComment(ctx, repo.ID, commit.ID.String(), comment); err != nil {
		return err
	}

	mentions, err := issues_model.ResolveRepoMentionsByVisibility(ctx, repo, doer, unit.TypeCode, references.FindAllMentionsMarkdown(content))
	if err != nil {
		return err
	}
	notify_service.CreateCommitComment(ctx, doer, repo, commit, comment, slices.DeleteFunc(mentions, func(user *user_model.User) bool {
		return user_model.IsUserBlockedBy(ctx, doer, user.ID)
	}))
	return nil
}
