// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/automerge"
)

// CreateCommitStatus creates a new CommitStatus given a bunch of parameters
// NOTE: All text-values will be trimmed from whitespaces.
// Requires: Repo, Creator, SHA
func CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, creator *user_model.User, sha string, status *git_model.CommitStatus) error {
	repoPath := repo.RepoPath()

	// confirm that commit is exist
	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return fmt.Errorf("OpenRepository[%s]: %w", repoPath, err)
	}
	defer closer.Close()

	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)

	commit, err := gitRepo.GetCommit(sha)
	if err != nil {
		gitRepo.Close()
		return fmt.Errorf("GetCommit[%s]: %w", sha, err)
	} else if len(sha) != objectFormat.FullLength() {
		// use complete commit sha
		sha = commit.ID.String()
	}
	gitRepo.Close()

	if err := git_model.NewCommitStatus(ctx, git_model.NewCommitStatusOptions{
		Repo:         repo,
		Creator:      creator,
		SHA:          commit.ID,
		CommitStatus: status,
	}); err != nil {
		return fmt.Errorf("NewCommitStatus[repo_id: %d, user_id: %d, sha: %s]: %w", repo.ID, creator.ID, sha, err)
	}

	if status.State.IsSuccess() {
		if err := automerge.MergeScheduledPullRequest(ctx, sha, repo); err != nil {
			return fmt.Errorf("MergeScheduledPullRequest[repo_id: %d, user_id: %d, sha: %s]: %w", repo.ID, creator.ID, sha, err)
		}
	}

	return nil
}

// CountDivergingCommits determines how many commits a branch is ahead or behind the repository's base branch
func CountDivergingCommits(ctx context.Context, repo *repo_model.Repository, branch string) (*git.DivergeObject, error) {
	divergence, err := git.GetDivergingCommits(ctx, repo.RepoPath(), repo.DefaultBranch, branch)
	if err != nil {
		return nil, err
	}
	return &divergence, nil
}

// GetPayloadCommitVerification returns the verification information of a commit
func GetPayloadCommitVerification(ctx context.Context, commit *git.Commit) *structs.PayloadCommitVerification {
	verification := &structs.PayloadCommitVerification{}
	commitVerification := asymkey_model.ParseCommitWithSignature(ctx, commit)
	if commit.Signature != nil {
		verification.Signature = commit.Signature.Signature
		verification.Payload = commit.Signature.Payload
	}
	if commitVerification.SigningUser != nil {
		verification.Signer = &structs.PayloadUser{
			Name:  commitVerification.SigningUser.Name,
			Email: commitVerification.SigningUser.Email,
		}
	}
	verification.Verified = commitVerification.Verified
	verification.Reason = commitVerification.Reason
	if verification.Reason == "" && !verification.Verified {
		verification.Reason = "gpg.error.not_signed_commit"
	}
	return verification
}
