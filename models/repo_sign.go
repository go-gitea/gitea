// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/login"
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

// SigningKey returns the KeyID and git Signature for the repo
func SigningKey(repoPath string) (string, *git.Signature) {
	if setting.Repository.Signing.SigningKey == "none" {
		return "", nil
	}

	if setting.Repository.Signing.SigningKey == "default" || setting.Repository.Signing.SigningKey == "" {
		// Can ignore the error here as it means that commit.gpgsign is not set
		value, _ := git.NewCommand("config", "--get", "commit.gpgsign").RunInDir(repoPath)
		sign, valid := git.ParseBool(strings.TrimSpace(value))
		if !sign || !valid {
			return "", nil
		}

		signingKey, _ := git.NewCommand("config", "--get", "user.signingkey").RunInDir(repoPath)
		signingName, _ := git.NewCommand("config", "--get", "user.name").RunInDir(repoPath)
		signingEmail, _ := git.NewCommand("config", "--get", "user.email").RunInDir(repoPath)
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
func PublicSigningKey(repoPath string) (string, error) {
	signingKey, _ := SigningKey(repoPath)
	if signingKey == "" {
		return "", nil
	}

	content, stderr, err := process.GetManager().ExecDir(-1, repoPath,
		"gpg --export -a", "gpg", "--export", "-a", signingKey)
	if err != nil {
		log.Error("Unable to get default signing key in %s: %s, %s, %v", repoPath, signingKey, stderr, err)
		return "", err
	}
	return content, nil
}

// SignInitialCommit determines if we should sign the initial commit to this repository
func SignInitialCommit(repoPath string, u *user_model.User) (bool, string, *git.Signature, error) {
	rules := signingModeFromStrings(setting.Repository.Signing.InitialCommit)
	signingKey, sig := SigningKey(repoPath)
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
			keys, err := ListGPGKeys(u.ID, db.ListOptions{})
			if err != nil {
				return false, "", nil, err
			}
			if len(keys) == 0 {
				return false, "", nil, &ErrWontSign{pubkey}
			}
		case twofa:
			twofaModel, err := login.GetTwoFactorByUID(u.ID)
			if err != nil && !login.IsErrTwoFactorNotEnrolled(err) {
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
func (repo *Repository) SignWikiCommit(u *user_model.User) (bool, string, *git.Signature, error) {
	rules := signingModeFromStrings(setting.Repository.Signing.Wiki)
	signingKey, sig := SigningKey(repo.WikiPath())
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
			keys, err := ListGPGKeys(u.ID, db.ListOptions{})
			if err != nil {
				return false, "", nil, err
			}
			if len(keys) == 0 {
				return false, "", nil, &ErrWontSign{pubkey}
			}
		case twofa:
			twofaModel, err := login.GetTwoFactorByUID(u.ID)
			if err != nil && !login.IsErrTwoFactorNotEnrolled(err) {
				return false, "", nil, err
			}
			if twofaModel == nil {
				return false, "", nil, &ErrWontSign{twofa}
			}
		case parentSigned:
			gitRepo, err := git.OpenRepository(repo.WikiPath())
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
			verification := ParseCommitWithSignature(commit)
			if !verification.Verified {
				return false, "", nil, &ErrWontSign{parentSigned}
			}
		}
	}
	return true, signingKey, sig, nil
}

// SignCRUDAction determines if we should sign a CRUD commit to this repository
func (repo *Repository) SignCRUDAction(u *user_model.User, tmpBasePath, parentCommit string) (bool, string, *git.Signature, error) {
	rules := signingModeFromStrings(setting.Repository.Signing.CRUDActions)
	signingKey, sig := SigningKey(repo.RepoPath())
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
			keys, err := ListGPGKeys(u.ID, db.ListOptions{})
			if err != nil {
				return false, "", nil, err
			}
			if len(keys) == 0 {
				return false, "", nil, &ErrWontSign{pubkey}
			}
		case twofa:
			twofaModel, err := login.GetTwoFactorByUID(u.ID)
			if err != nil && !login.IsErrTwoFactorNotEnrolled(err) {
				return false, "", nil, err
			}
			if twofaModel == nil {
				return false, "", nil, &ErrWontSign{twofa}
			}
		case parentSigned:
			gitRepo, err := git.OpenRepository(tmpBasePath)
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
			verification := ParseCommitWithSignature(commit)
			if !verification.Verified {
				return false, "", nil, &ErrWontSign{parentSigned}
			}
		}
	}
	return true, signingKey, sig, nil
}
