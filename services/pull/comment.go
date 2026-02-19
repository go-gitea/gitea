// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	git_service "code.gitea.io/gitea/services/git"
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

type CachedCommitUser struct {
	ID       int64
	Name     string
	FullName string
	Email    string
	Avatar   string
}

func convertUserToCachedCommitUser(u *user_model.User) *CachedCommitUser {
	if u == nil {
		return nil
	}
	return &CachedCommitUser{
		ID:       u.ID,
		Name:     u.Name,
		FullName: u.FullName,
		Email:    u.Email,
		Avatar:   u.Avatar,
	}
}

func convertCachedUserToUser(cu *CachedCommitUser) *user_model.User {
	if cu == nil {
		return nil
	}
	return &user_model.User{
		ID:       cu.ID,
		Name:     cu.Name,
		FullName: cu.FullName,
		Email:    cu.Email,
		Avatar:   cu.Avatar,
	}
}

// CachedCommit will be stored in database as part of push comment content to reduce
// disk read when loading push commits later.
// it will only keep necessary information to display in the timeline of the pull request
type CachedCommit struct {
	CommitID      string
	Author        *git.Signature
	Committer     *git.Signature
	CommitMessage string
	User          *CachedCommitUser

	Verified                 bool
	Warning                  bool
	Reason                   string
	SigningUser              *CachedCommitUser // if Verified, then SigningUser is non-nil
	CommittingUser           *CachedCommitUser // if Verified, then CommittingUser is non-nil
	SigningEmail             string
	SigningGPGKeyID          string
	SigningSSHKeyFingerprint string
	TrustStatus              string
}

func convertCachedCommitsToGitCommits(cachedCommits []CachedCommit) []*asymkey_model.SignCommit {
	var gitCommits []*asymkey_model.SignCommit
	for _, cc := range cachedCommits {
		objectID := git.MustIDFromString(cc.CommitID)
		signedCommit := &asymkey_model.SignCommit{
			UserCommit: &user_model.UserCommit{
				User: convertCachedUserToUser(cc.User),
				Commit: &git.Commit{
					ID:            objectID,
					Author:        cc.Author,
					Committer:     cc.Committer,
					CommitMessage: cc.CommitMessage,
				},
			},
			Verification: &asymkey_model.CommitVerification{
				Verified:       cc.Verified,
				Warning:        cc.Warning,
				Reason:         cc.Reason,
				SigningEmail:   cc.SigningEmail,
				TrustStatus:    cc.TrustStatus,
				SigningUser:    convertCachedUserToUser(cc.SigningUser),
				CommittingUser: convertCachedUserToUser(cc.CommittingUser),
			},
		}

		if cc.SigningGPGKeyID != "" {
			signedCommit.Verification.SigningKey = &asymkey_model.GPGKey{
				KeyID: cc.SigningGPGKeyID,
			}
		} else if cc.SigningSSHKeyFingerprint != "" {
			signedCommit.Verification.SigningSSHKey = &asymkey_model.PublicKey{
				Fingerprint: cc.SigningSSHKeyFingerprint,
			}
		}

		gitCommits = append(gitCommits, signedCommit)
	}
	return gitCommits
}

func convertGitCommitsToCachedCommits(commits []*asymkey_model.SignCommit) []CachedCommit {
	var cachedCommits []CachedCommit
	for _, c := range commits {
		var signingGPGKeyID, signingSSHKeyFingerprint string
		if c.Verification != nil {
			if c.Verification.SigningKey != nil {
				signingGPGKeyID = c.Verification.SigningKey.KeyID
			} else if c.Verification.SigningSSHKey != nil {
				signingSSHKeyFingerprint = c.Verification.SigningSSHKey.Fingerprint
			}
		}
		cachedCommits = append(cachedCommits, CachedCommit{
			CommitID:      c.ID.String(),
			Author:        c.Author,
			Committer:     c.Committer,
			CommitMessage: c.CommitMessage,
			User:          convertUserToCachedCommitUser(c.User),

			Verified:                 c.Verification.Verified,
			Warning:                  c.Verification.Warning,
			Reason:                   c.Verification.Reason,
			SigningUser:              convertUserToCachedCommitUser(c.Verification.SigningUser),
			CommittingUser:           convertUserToCachedCommitUser(c.Verification.CommittingUser),
			SigningEmail:             c.Verification.SigningEmail,
			SigningGPGKeyID:          signingGPGKeyID,
			SigningSSHKeyFingerprint: signingSSHKeyFingerprint,
			TrustStatus:              c.Verification.TrustStatus,
		})
	}
	return cachedCommits
}

// PushActionContent is content of push pull comment
type PushActionContent struct {
	IsForcePush   bool           `json:"is_force_push"`
	CommitIDs     []string       `json:"commit_ids"`
	CachedCommits []CachedCommit `json:"cached_commits"`
}

// CreatePushPullComment create push code to pull base comment
func CreatePushPullComment(ctx context.Context, pusher *user_model.User, pr *issues_model.PullRequest, oldCommitID, newCommitID string, isForcePush bool) (comment *issues_model.Comment, err error) {
	if pr.HasMerged || oldCommitID == "" || newCommitID == "" {
		return nil, nil //nolint:nilnil // return nil because no comment needs to be created
	}

	opts := &issues_model.CreateCommentOptions{
		Type:        issues_model.CommentTypePullRequestPush,
		Doer:        pusher,
		Repo:        pr.BaseRepo,
		IsForcePush: isForcePush,
		Issue:       pr.Issue,
	}

	var data PushActionContent
	if opts.IsForcePush {
		data.CommitIDs = []string{oldCommitID, newCommitID}
		data.IsForcePush = true
	} else {
		data.CommitIDs, err = getCommitIDsFromRepo(ctx, pr.BaseRepo, oldCommitID, newCommitID, pr.BaseBranch)
		if err != nil {
			return nil, err
		}
		// It maybe an empty pull request. Only non-empty pull request need to create push comment
		if len(data.CommitIDs) == 0 {
			return nil, nil //nolint:nilnil // return nil because no comment needs to be created
		}

		gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, pr.BaseRepo)
		if err != nil {
			return nil, err
		}
		defer closer.Close()

		validatedCommits, err := user_model.ValidateCommitsWithEmails(ctx, gitRepo.GetCommitsFromIDs(data.CommitIDs))
		if err != nil {
			return nil, err
		}
		signedCommits, err := git_service.ParseCommitsWithSignature(
			ctx,
			pr.BaseRepo,
			validatedCommits,
			pr.BaseRepo.GetTrustModel(),
		)
		if err != nil {
			return nil, err
		}

		data.CachedCommits = convertGitCommitsToCachedCommits(signedCommits)
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	opts.Content = string(dataJSON)
	comment, err = issues_model.CreateComment(ctx, opts)

	return comment, err
}

// LoadCommentPushCommits Load push commits
func LoadCommentPushCommits(ctx context.Context, c *issues_model.Comment) error {
	if c.Content == "" || c.Commits != nil || c.Type != issues_model.CommentTypePullRequestPush {
		return nil
	}

	var data PushActionContent
	if err := json.Unmarshal([]byte(c.Content), &data); err != nil {
		log.Debug("Unmarshal: %v", err) // no need to show 500 error to end user when the JSON is broken
		return nil
	}

	c.IsForcePush = data.IsForcePush

	if c.IsForcePush {
		if len(data.CommitIDs) != 2 {
			return nil
		}
		c.OldCommit, c.NewCommit = data.CommitIDs[0], data.CommitIDs[1]
	} else {
		if err := c.LoadIssue(ctx); err != nil {
			return err
		}
		if err := c.Issue.LoadRepo(ctx); err != nil {
			return err
		}

		gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, c.Issue.Repo)
		if err != nil {
			return err
		}
		defer closer.Close()

		if data.CachedCommits != nil {
			convertedCommits := convertCachedCommitsToGitCommits(data.CachedCommits)
			c.Commits, err = git_service.ParseCommitsWithStatus(ctx, convertedCommits, c.Issue.Repo)
		} else {
			c.Commits, err = git_service.ConvertFromGitCommit(ctx, gitRepo.GetCommitsFromIDs(data.CommitIDs), c.Issue.Repo)
		}

		if err != nil {
			log.Debug("ConvertFromGitCommit: %v", err) // no need to show 500 error to end user when the commit does not exist
		} else {
			c.CommitsNum = int64(len(c.Commits))
		}
	}

	return nil
}
