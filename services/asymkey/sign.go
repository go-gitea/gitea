// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
)

type signingMode string

const (
	never         signingMode = "never"
	always        signingMode = "always"
	pubkey        signingMode = "pubkey"
	twofa         signingMode = "twofa"
	parentSigned  signingMode = "parentsigned"
	baseSigned    signingMode = "basesigned"
	headSigned    signingMode = "headsigned"
	commitsSigned signingMode = "commitssigned"
	approved      signingMode = "approved"
	noKey         signingMode = "nokey"
)

func signingModeFromStrings(modeStrings []string) []signingMode {
	returnable := make([]signingMode, 0, len(modeStrings))
	for _, mode := range modeStrings {
		signMode := signingMode(strings.ToLower(strings.TrimSpace(mode)))
		switch signMode {
		case never:
			return []signingMode{never}
		case always:
			return []signingMode{always}
		case pubkey:
			fallthrough
		case twofa:
			fallthrough
		case parentSigned:
			fallthrough
		case baseSigned:
			fallthrough
		case headSigned:
			fallthrough
		case approved:
			fallthrough
		case commitsSigned:
			returnable = append(returnable, signMode)
		}
	}
	if len(returnable) == 0 {
		return []signingMode{never}
	}
	return returnable
}

// ErrWontSign explains the first reason why a commit would not be signed
// There may be other reasons - this is just the first reason found
type ErrWontSign struct {
	Reason signingMode
}

func (e *ErrWontSign) Error() string {
	return fmt.Sprintf("wont sign: %s", e.Reason)
}

// IsErrWontSign checks if an error is a ErrWontSign
func IsErrWontSign(err error) bool {
	_, ok := err.(*ErrWontSign)
	return ok
}

// SigningKey returns the KeyID and git Signature for the repo
func SigningKey(ctx context.Context, repoPath string) (string, *git.Signature) {
	if setting.Repository.Signing.SigningKey == "none" {
		return "", nil
	}

	if setting.Repository.Signing.SigningKey == "default" || setting.Repository.Signing.SigningKey == "" {
		// Can ignore the error here as it means that commit.gpgsign is not set
		value, _, _ := git.NewCommand(ctx, "config", "--get", "commit.gpgsign").RunStdString(&git.RunOpts{Dir: repoPath})
		sign, valid := git.ParseBool(strings.TrimSpace(value))
		if !sign || !valid {
			return "", nil
		}

		signingKey, _, _ := git.NewCommand(ctx, "config", "--get", "user.signingkey").RunStdString(&git.RunOpts{Dir: repoPath})
		signingName, _, _ := git.NewCommand(ctx, "config", "--get", "user.name").RunStdString(&git.RunOpts{Dir: repoPath})
		signingEmail, _, _ := git.NewCommand(ctx, "config", "--get", "user.email").RunStdString(&git.RunOpts{Dir: repoPath})
		return strings.TrimSpace(signingKey), &git.Signature{
			Name:  strings.TrimSpace(signingName),
			Email: strings.TrimSpace(signingEmail),
		}
	}

	return setting.Repository.Signing.SigningKey, &git.Signature{
		Name:  setting.Repository.Signing.SigningName,
		Email: setting.Repository.Signing.SigningEmail,
	}
}

// PublicSigningKey gets the public signing key within a provided repository directory
func PublicSigningKey(ctx context.Context, repoPath string) (string, error) {
	signingKey, _ := SigningKey(ctx, repoPath)
	if signingKey == "" {
		return "", nil
	}

	content, stderr, err := process.GetManager().ExecDir(ctx, -1, repoPath,
		"gpg --export -a", "gpg", "--export", "-a", signingKey)
	if err != nil {
		log.Error("Unable to get default signing key in %s: %s, %s, %v", repoPath, signingKey, stderr, err)
		return "", err
	}
	return content, nil
}

// SignInitialCommit determines if we should sign the initial commit to this repository
func SignInitialCommit(ctx context.Context, repoPath string, u *user_model.User) (bool, string, *git.Signature, error) {
	rules := signingModeFromStrings(setting.Repository.Signing.InitialCommit)
	signingKey, sig := SigningKey(ctx, repoPath)
	if signingKey == "" {
		return false, "", nil, &ErrWontSign{noKey}
	}

Loop:
	for _, rule := range rules {
		switch rule {
		case never:
			return false, "", nil, &ErrWontSign{never}
		case always:
			break Loop
		case pubkey:
			keys, err := asymkey_model.ListGPGKeys(ctx, u.ID, db.ListOptions{})
			if err != nil {
				return false, "", nil, err
			}
			if len(keys) == 0 {
				return false, "", nil, &ErrWontSign{pubkey}
			}
		case twofa:
			twofaModel, err := auth.GetTwoFactorByUID(ctx, u.ID)
			if err != nil && !auth.IsErrTwoFactorNotEnrolled(err) {
				return false, "", nil, err
			}
			if twofaModel == nil {
				return false, "", nil, &ErrWontSign{twofa}
			}
		}
	}
	return true, signingKey, sig, nil
}

// SignWikiCommit determines if we should sign the commits to this repository wiki
func SignWikiCommit(ctx context.Context, repoWikiPath string, u *user_model.User) (bool, string, *git.Signature, error) {
	rules := signingModeFromStrings(setting.Repository.Signing.Wiki)
	signingKey, sig := SigningKey(ctx, repoWikiPath)
	if signingKey == "" {
		return false, "", nil, &ErrWontSign{noKey}
	}

Loop:
	for _, rule := range rules {
		switch rule {
		case never:
			return false, "", nil, &ErrWontSign{never}
		case always:
			break Loop
		case pubkey:
			keys, err := asymkey_model.ListGPGKeys(ctx, u.ID, db.ListOptions{})
			if err != nil {
				return false, "", nil, err
			}
			if len(keys) == 0 {
				return false, "", nil, &ErrWontSign{pubkey}
			}
		case twofa:
			twofaModel, err := auth.GetTwoFactorByUID(ctx, u.ID)
			if err != nil && !auth.IsErrTwoFactorNotEnrolled(err) {
				return false, "", nil, err
			}
			if twofaModel == nil {
				return false, "", nil, &ErrWontSign{twofa}
			}
		case parentSigned:
			gitRepo, err := git.OpenRepository(ctx, repoWikiPath)
			if err != nil {
				return false, "", nil, err
			}
			defer gitRepo.Close()
			commit, err := gitRepo.GetCommit("HEAD")
			if err != nil {
				return false, "", nil, err
			}
			if commit.Signature == nil {
				return false, "", nil, &ErrWontSign{parentSigned}
			}
			verification := asymkey_model.ParseCommitWithSignature(ctx, commit)
			if !verification.Verified {
				return false, "", nil, &ErrWontSign{parentSigned}
			}
		}
	}
	return true, signingKey, sig, nil
}

// SignCRUDAction determines if we should sign a CRUD commit to this repository
func SignCRUDAction(ctx context.Context, repoPath string, u *user_model.User, tmpBasePath, parentCommit string) (bool, string, *git.Signature, error) {
	rules := signingModeFromStrings(setting.Repository.Signing.CRUDActions)
	signingKey, sig := SigningKey(ctx, repoPath)
	if signingKey == "" {
		return false, "", nil, &ErrWontSign{noKey}
	}

Loop:
	for _, rule := range rules {
		switch rule {
		case never:
			return false, "", nil, &ErrWontSign{never}
		case always:
			break Loop
		case pubkey:
			keys, err := asymkey_model.ListGPGKeys(ctx, u.ID, db.ListOptions{})
			if err != nil {
				return false, "", nil, err
			}
			if len(keys) == 0 {
				return false, "", nil, &ErrWontSign{pubkey}
			}
		case twofa:
			twofaModel, err := auth.GetTwoFactorByUID(ctx, u.ID)
			if err != nil && !auth.IsErrTwoFactorNotEnrolled(err) {
				return false, "", nil, err
			}
			if twofaModel == nil {
				return false, "", nil, &ErrWontSign{twofa}
			}
		case parentSigned:
			gitRepo, err := git.OpenRepository(ctx, tmpBasePath)
			if err != nil {
				return false, "", nil, err
			}
			defer gitRepo.Close()
			commit, err := gitRepo.GetCommit(parentCommit)
			if err != nil {
				return false, "", nil, err
			}
			if commit.Signature == nil {
				return false, "", nil, &ErrWontSign{parentSigned}
			}
			verification := asymkey_model.ParseCommitWithSignature(ctx, commit)
			if !verification.Verified {
				return false, "", nil, &ErrWontSign{parentSigned}
			}
		}
	}
	return true, signingKey, sig, nil
}

// SignMerge determines if we should sign a PR merge commit to the base repository
func SignMerge(ctx context.Context, pr *issues_model.PullRequest, u *user_model.User, tmpBasePath, baseCommit, headCommit string) (bool, string, *git.Signature, error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("Unable to get Base Repo for pull request")
		return false, "", nil, err
	}
	repo := pr.BaseRepo

	signingKey, signer := SigningKey(ctx, repo.RepoPath())
	if signingKey == "" {
		return false, "", nil, &ErrWontSign{noKey}
	}
	rules := signingModeFromStrings(setting.Repository.Signing.Merges)

	var gitRepo *git.Repository
	var err error

Loop:
	for _, rule := range rules {
		switch rule {
		case never:
			return false, "", nil, &ErrWontSign{never}
		case always:
			break Loop
		case pubkey:
			keys, err := asymkey_model.ListGPGKeys(ctx, u.ID, db.ListOptions{})
			if err != nil {
				return false, "", nil, err
			}
			if len(keys) == 0 {
				return false, "", nil, &ErrWontSign{pubkey}
			}
		case twofa:
			twofaModel, err := auth.GetTwoFactorByUID(ctx, u.ID)
			if err != nil && !auth.IsErrTwoFactorNotEnrolled(err) {
				return false, "", nil, err
			}
			if twofaModel == nil {
				return false, "", nil, &ErrWontSign{twofa}
			}
		case approved:
			protectedBranch, err := git_model.GetFirstMatchProtectedBranchRule(ctx, repo.ID, pr.BaseBranch)
			if err != nil {
				return false, "", nil, err
			}
			if protectedBranch == nil {
				return false, "", nil, &ErrWontSign{approved}
			}
			if issues_model.GetGrantedApprovalsCount(ctx, protectedBranch, pr) < 1 {
				return false, "", nil, &ErrWontSign{approved}
			}
		case baseSigned:
			if gitRepo == nil {
				gitRepo, err = git.OpenRepository(ctx, tmpBasePath)
				if err != nil {
					return false, "", nil, err
				}
				defer gitRepo.Close()
			}
			commit, err := gitRepo.GetCommit(baseCommit)
			if err != nil {
				return false, "", nil, err
			}
			verification := asymkey_model.ParseCommitWithSignature(ctx, commit)
			if !verification.Verified {
				return false, "", nil, &ErrWontSign{baseSigned}
			}
		case headSigned:
			if gitRepo == nil {
				gitRepo, err = git.OpenRepository(ctx, tmpBasePath)
				if err != nil {
					return false, "", nil, err
				}
				defer gitRepo.Close()
			}
			commit, err := gitRepo.GetCommit(headCommit)
			if err != nil {
				return false, "", nil, err
			}
			verification := asymkey_model.ParseCommitWithSignature(ctx, commit)
			if !verification.Verified {
				return false, "", nil, &ErrWontSign{headSigned}
			}
		case commitsSigned:
			if gitRepo == nil {
				gitRepo, err = git.OpenRepository(ctx, tmpBasePath)
				if err != nil {
					return false, "", nil, err
				}
				defer gitRepo.Close()
			}
			commit, err := gitRepo.GetCommit(headCommit)
			if err != nil {
				return false, "", nil, err
			}
			verification := asymkey_model.ParseCommitWithSignature(ctx, commit)
			if !verification.Verified {
				return false, "", nil, &ErrWontSign{commitsSigned}
			}
			// need to work out merge-base
			mergeBaseCommit, _, err := gitRepo.GetMergeBase("", baseCommit, headCommit)
			if err != nil {
				return false, "", nil, err
			}
			commitList, err := commit.CommitsBeforeUntil(mergeBaseCommit)
			if err != nil {
				return false, "", nil, err
			}
			for _, commit := range commitList {
				verification := asymkey_model.ParseCommitWithSignature(ctx, commit)
				if !verification.Verified {
					return false, "", nil, &ErrWontSign{commitsSigned}
				}
			}
		}
	}
	return true, signingKey, signer, nil
}
