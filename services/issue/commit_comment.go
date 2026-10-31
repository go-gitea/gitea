// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"slices"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/references"
	notify_service "gitea.dev/services/notify"
)

func commitBlockerIDs(ctx context.Context, repo *repo_model.Repository, commit *git.Commit) []int64 {
	blockerIDs := []int64{repo.OwnerID}
	if author := user_model.GetUserByGitAuthor(ctx, commit); author != nil {
		blockerIDs = append(blockerIDs, author.ID)
	}
	return blockerIDs
}

func commentBlockerIDs(ctx context.Context, repo *repo_model.Repository, comment *issues_model.Comment) ([]int64, error) {
	if !comment.IsCommitComment() {
		if err := comment.LoadIssue(ctx); err != nil {
			return nil, err
		}
		return []int64{repo.OwnerID, comment.Issue.PosterID}, nil
	}
	if err := comment.LoadCommitComment(ctx); err != nil {
		return nil, err
	}
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	commit, err := gitRepo.GetCommit(ctx, comment.CommitComment.CommitSHA)
	if err != nil {
		return nil, err
	}
	return commitBlockerIDs(ctx, repo, commit), nil
}

func CreateCommitComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, commit *git.Commit, treePath string, line int64, content string, attachmentUUIDs []string) error {
	if user_model.IsUserBlockedBy(ctx, doer, commitBlockerIDs(ctx, repo, commit)...) && !access_model.IsUserRepoAdmin(ctx, repo, doer) {
		return user_model.ErrBlockedUser
	}

	comment := &issues_model.Comment{
		PosterID: doer.ID,
		Poster:   doer,
		TreePath: treePath,
		Line:     line,
		Content:  content,
	}
	if err := issues_model.CreateCommitComment(ctx, repo.ID, commit.ID.String(), comment, attachmentUUIDs); err != nil {
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
