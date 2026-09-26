// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	asymkey_model "gitea.dev/models/asymkey"
	git_model "gitea.dev/models/git"
	"gitea.dev/models/gituser"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	asymkey_service "gitea.dev/services/asymkey"
)

// ParseCommitsWithSignature checks if signaute of commits are corresponding to users gpg keys.
func ParseCommitsWithSignature(ctx context.Context, repo *repo_model.Repository, oldCommits []*gituser.UserCommit, repoTrustModel repo_model.TrustModelType) ([]*asymkey_model.SignCommit, error) {
	newCommits := make([]*asymkey_model.SignCommit, 0, len(oldCommits))
	keyMap := map[string]bool{}

	emails := make(container.Set[string])
	for _, c := range oldCommits {
		if c.GitCommit.Committer != nil {
			emails.Add(c.GitCommit.Committer.Email)
		}
	}

	emailUsers, err := user_model.GetUsersByEmails(ctx, emails.Values())
	if err != nil {
		return nil, err
	}

	for _, c := range oldCommits {
		committerUser := emailUsers.GetByEmail(c.GitCommit.Committer.Email) // FIXME: why GetUserCommitsByGitCommits uses "Author", but ParseCommitsWithSignature uses "Committer"?
		signCommit := &asymkey_model.SignCommit{
			UserCommit:   c,
			Verification: asymkey_service.ParseCommitWithSignatureCommitter(ctx, c.GitCommit, committerUser),
		}

		isOwnerMemberCollaborator := func(user *user_model.User) (bool, error) {
			return repo_model.HasAccessToRepoCodeUnit(ctx, repo, user.ID)
		}

		_ = asymkey_model.CalculateTrustStatus(signCommit.Verification, repoTrustModel, isOwnerMemberCollaborator, &keyMap)

		newCommits = append(newCommits, signCommit)
	}
	return newCommits, nil
}

// ConvertFromGitCommit converts git commits into SignCommitWithStatuses
func ConvertFromGitCommit(ctx context.Context, commits []*git.Commit, repo *repo_model.Repository, currentRef git.RefName) ([]*git_model.SignCommitWithStatuses, error) {
	userCommits, err := gituser.GetUserCommitsByGitCommits(ctx, commits, repo.Link(), currentRef)
	if err != nil {
		return nil, err
	}
	signedCommits, err := ParseCommitsWithSignature(
		ctx,
		repo,
		userCommits,
		repo.GetTrustModel(),
	)
	if err != nil {
		return nil, err
	}
	return ParseCommitsWithStatus(ctx, signedCommits, repo)
}

// maxCommitStatusLookupCount bounds the commit status lookup: the compare and pull request commit
// pages are not paginated, so an unrelated-histories comparison would otherwise ask the database
// for the statuses of every commit in the repository. The lookup runs one batch per 50 commits at
// well under a millisecond each, so this is set high enough never to drop a status for a real
// comparison, and still far below what verifying the same commits' signatures costs.
const maxCommitStatusLookupCount = 5000

// ParseCommitsWithStatus checks commits latest statuses and calculates its worst status state
func ParseCommitsWithStatus(ctx context.Context, oldCommits []*asymkey_model.SignCommit, repo *repo_model.Repository) ([]*git_model.SignCommitWithStatuses, error) {
	if len(oldCommits) == 0 {
		return nil, nil
	}

	var statusMap map[string][]*git_model.CommitStatus
	if len(oldCommits) <= maxCommitStatusLookupCount {
		commitIDs := make([]string, 0, len(oldCommits))
		for _, c := range oldCommits {
			commitIDs = append(commitIDs, c.GitCommit.ID.String())
		}
		var err error
		statusMap, err = git_model.GetLatestCommitStatusForRepoCommitIDs(ctx, repo.ID, commitIDs)
		if err != nil {
			return nil, err
		}
	}

	newCommits := make([]*git_model.SignCommitWithStatuses, 0, len(oldCommits))
	for _, c := range oldCommits {
		statuses := statusMap[c.GitCommit.ID.String()]
		newCommits = append(newCommits, &git_model.SignCommitWithStatuses{
			SignCommit: c,
			Statuses:   statuses,
			Status:     git_model.CalcCommitStatus(statuses),
		})
	}
	return newCommits, nil
}
