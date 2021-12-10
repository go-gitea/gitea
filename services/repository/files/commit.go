// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package files

import (
	"fmt"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/structs"
)

// CreateCommitStatus creates a new CommitStatus given a bunch of parameters
// NOTE: All text-values will be trimmed from whitespaces.
// Requires: Repo, Creator, SHA
func CreateCommitStatus(repo *repo_model.Repository, creator *user_model.User, sha string, status *models.CommitStatus) error {
	repoPath := repo.RepoPath()

	// confirm that commit is exist
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return fmt.Errorf("OpenRepository[%s]: %v", repoPath, err)
	}
	if _, err := gitRepo.GetCommit(sha); err != nil {
		gitRepo.Close()
		return fmt.Errorf("GetCommit[%s]: %v", sha, err)
	}
	gitRepo.Close()

	if err := models.NewCommitStatus(models.NewCommitStatusOptions{
		Repo:         repo,
		Creator:      creator,
		SHA:          sha,
		CommitStatus: status,
	}); err != nil {
		return fmt.Errorf("NewCommitStatus[repo_id: %d, user_id: %d, sha: %s]: %v", repo.ID, creator.ID, sha, err)
	}

	return nil
}

// CountDivergingCommits determines how many commits a branch is ahead or behind the repository's base branch
func CountDivergingCommits(repo *repo_model.Repository, branch string) (*git.DivergeObject, error) {
	divergence, err := git.GetDivergingCommits(repo.RepoPath(), repo.DefaultBranch, branch)
	if err != nil {
		return nil, err
	}
	return &divergence, nil
}

// GetPayloadCommitVerification returns the verification information of a commit
func GetPayloadCommitVerification(commit *git.Commit) *structs.PayloadCommitVerification {
	verification := &structs.PayloadCommitVerification{}
	commitVerification := models.ParseCommitWithSignature(commit)
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
