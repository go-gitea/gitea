// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"

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
)

func signingModeFromStrings(modeStrings []string) []signingMode {
	returnable := make([]signingMode, 0, len(modeStrings))
	for _, mode := range modeStrings {
		signMode := signingMode(strings.ToLower(mode))
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

func signingKey(repoPath string) string {
	if setting.Repository.Signing.SigningKey == "none" {
		return ""
	}

	if setting.Repository.Signing.SigningKey == "default" || setting.Repository.Signing.SigningKey == "" {
		// Can ignore the error here as it means that commit.gpgsign is not set
		value, _ := git.NewCommand("config", "--get", "commit.gpgsign").RunInDir(repoPath)
		sign, valid := git.ParseBool(strings.TrimSpace(value))
		if !sign || !valid {
			return ""
		}

		signingKey, _ := git.NewCommand("config", "--get", "user.signingkey").RunInDir(repoPath)
		return strings.TrimSpace(signingKey)
	}

	return setting.Repository.Signing.SigningKey
}

// PublicSigningKey gets the public signing key within a provided repository directory
func PublicSigningKey(repoPath string) (string, error) {
	signingKey := signingKey(repoPath)
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
func SignInitialCommit(repoPath string, u *User) (bool, string) {
	rules := signingModeFromStrings(setting.Repository.Signing.InitialCommit)
	signingKey := signingKey(repoPath)
	if signingKey == "" {
		return false, ""
	}

	for _, rule := range rules {
		switch rule {
		case never:
			return false, ""
		case always:
			break
		case pubkey:
			keys, err := ListGPGKeys(u.ID)
			if err != nil || len(keys) == 0 {
				return false, ""
			}
		case twofa:
			twofa, err := GetTwoFactorByUID(u.ID)
			if err != nil || twofa == nil {
				return false, ""
			}
		}
	}
	return true, signingKey
}

// SignWikiCommit determines if we should sign the commits to this repository wiki
func (repo *Repository) SignWikiCommit(u *User) (bool, string) {
	rules := signingModeFromStrings(setting.Repository.Signing.Wiki)
	signingKey := signingKey(repo.WikiPath())
	if signingKey == "" {
		return false, ""
	}

	for _, rule := range rules {
		switch rule {
		case never:
			return false, ""
		case always:
			break
		case pubkey:
			keys, err := ListGPGKeys(u.ID)
			if err != nil || len(keys) == 0 {
				return false, ""
			}
		case twofa:
			twofa, err := GetTwoFactorByUID(u.ID)
			if err != nil || twofa == nil {
				return false, ""
			}
		case parentSigned:
			gitRepo, err := git.OpenRepository(repo.WikiPath())
			if err != nil {
				return false, ""
			}
			defer gitRepo.Close()
			commit, err := gitRepo.GetCommit("HEAD")
			if err != nil {
				return false, ""
			}
			if commit.Signature == nil {
				return false, ""
			}
			verification := ParseCommitWithSignature(commit)
			if !verification.Verified {
				return false, ""
			}
		}
	}
	return true, signingKey
}

// SignCRUDAction determines if we should sign a CRUD commit to this repository
func (repo *Repository) SignCRUDAction(u *User, tmpBasePath, parentCommit string) (bool, string) {
	rules := signingModeFromStrings(setting.Repository.Signing.CRUDActions)
	signingKey := signingKey(repo.RepoPath())
	if signingKey == "" {
		return false, ""
	}

	for _, rule := range rules {
		switch rule {
		case never:
			return false, ""
		case always:
			break
		case pubkey:
			keys, err := ListGPGKeys(u.ID)
			if err != nil || len(keys) == 0 {
				return false, ""
			}
		case twofa:
			twofa, err := GetTwoFactorByUID(u.ID)
			if err != nil || twofa == nil {
				return false, ""
			}
		case parentSigned:
			gitRepo, err := git.OpenRepository(tmpBasePath)
			if err != nil {
				return false, ""
			}
			defer gitRepo.Close()
			commit, err := gitRepo.GetCommit(parentCommit)
			if err != nil {
				return false, ""
			}
			if commit.Signature == nil {
				return false, ""
			}
			verification := ParseCommitWithSignature(commit)
			if !verification.Verified {
				return false, ""
			}
		}
	}
	return true, signingKey
}
