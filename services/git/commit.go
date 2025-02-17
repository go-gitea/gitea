// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
)

// ParseCommitsWithSignature checks if signaute of commits are corresponding to users gpg keys.
func ParseCommitsWithSignature(ctx context.Context, oldCommits []*user_model.UserCommit, repoTrustModel repo_model.TrustModelType, isOwnerMemberCollaborator func(*user_model.User) (bool, error)) ([]*asymkey_model.SignCommit, error) {
	newCommits := make([]*asymkey_model.SignCommit, 0, len(oldCommits))
	keyMap := map[string]bool{}

	emails := make(container.Set[string])
	for _, c := range oldCommits {
		if c.Committer != nil {
			emails.Add(c.Committer.Email)
		}
	}

	emailUsers, err := user_model.GetUsersByEmails(ctx, emails.Values())
	if err != nil {
		return nil, err
	}

	for _, c := range oldCommits {
		committer, ok := emailUsers[c.Committer.Email]
		if !ok && c.Committer != nil {
			committer = &user_model.User{
				Name:  c.Committer.Name,
				Email: c.Committer.Email,
			}
		}

		signCommit := &asymkey_model.SignCommit{
			UserCommit:   c,
			Verification: asymkey_service.ParseCommitWithSignatureCommitter(ctx, c.Commit, committer),
		}

		_ = asymkey_model.CalculateTrustStatus(signCommit.Verification, repoTrustModel, isOwnerMemberCollaborator, &keyMap)

		newCommits = append(newCommits, signCommit)
	}
	return newCommits, nil
}

// ConvertFromGitCommit converts git commits into SignCommitWithStatuses
func ConvertFromGitCommit(ctx context.Context, commits []*git.Commit, repo *repo_model.Repository) ([]*git_model.SignCommitWithStatuses, error) {
	validatedCommits, err := user_model.ValidateCommitsWithEmails(ctx, commits)
	if err != nil {
		return nil, err
	}
	signedCommits, err := ParseCommitsWithSignature(
		ctx,
		validatedCommits,
		repo.GetTrustModel(),
		func(user *user_model.User) (bool, error) {
			return repo_model.IsOwnerMemberCollaborator(ctx, repo, user.ID)
		},
	)
	if err != nil {
		return nil, err
	}
	return ParseCommitsWithStatus(ctx, signedCommits, repo)
}

// ParseCommitsWithStatus checks commits latest statuses and calculates its worst status state
func ParseCommitsWithStatus(ctx context.Context, oldCommits []*asymkey_model.SignCommit, repo *repo_model.Repository) ([]*git_model.SignCommitWithStatuses, error) {
	newCommits := make([]*git_model.SignCommitWithStatuses, 0, len(oldCommits))

	for _, c := range oldCommits {
		commit := &git_model.SignCommitWithStatuses{
			SignCommit: c,
		}
		statuses, _, err := git_model.GetLatestCommitStatus(ctx, repo.ID, commit.ID.String(), db.ListOptions{})
		if err != nil {
			return nil, err
		}

		commit.Statuses = statuses
		commit.Status = git_model.CalcCommitStatus(statuses)
		newCommits = append(newCommits, commit)
	}
	return newCommits, nil
}
