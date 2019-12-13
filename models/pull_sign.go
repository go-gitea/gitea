// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// SignMerge determines if we should sign a PR merge commit to the base repository
func (pr *PullRequest) SignMerge(u *User, tmpBasePath, baseCommit, headCommit string) (bool, string) {
	if err := pr.GetBaseRepo(); err != nil {
		log.Error("Unable to get Base Repo for pull request")
		return false, ""
	}
	repo := pr.BaseRepo

	signingKey := signingKey(repo.RepoPath())
	if signingKey == "" {
		return false, ""
	}
	rules := signingModeFromStrings(setting.Repository.Signing.Merges)

	var gitRepo *git.Repository
	var err error

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
		case approved:
			protectedBranch, err := GetProtectedBranchBy(repo.ID, pr.BaseBranch)
			if err != nil || protectedBranch == nil {
				return false, ""
			}
			if protectedBranch.GetGrantedApprovalsCount(pr) < 1 {
				return false, ""
			}
		case baseSigned:
			if gitRepo == nil {
				gitRepo, err = git.OpenRepository(tmpBasePath)
				if err != nil {
					return false, ""
				}
				defer gitRepo.Close()
			}
			commit, err := gitRepo.GetCommit(baseCommit)
			if err != nil {
				return false, ""
			}
			verification := ParseCommitWithSignature(commit)
			if !verification.Verified {
				return false, ""
			}
		case headSigned:
			if gitRepo == nil {
				gitRepo, err = git.OpenRepository(tmpBasePath)
				if err != nil {
					return false, ""
				}
				defer gitRepo.Close()
			}
			commit, err := gitRepo.GetCommit(headCommit)
			if err != nil {
				return false, ""
			}
			verification := ParseCommitWithSignature(commit)
			if !verification.Verified {
				return false, ""
			}
		case commitsSigned:
			if gitRepo == nil {
				gitRepo, err = git.OpenRepository(tmpBasePath)
				if err != nil {
					return false, ""
				}
				defer gitRepo.Close()
			}
			commit, err := gitRepo.GetCommit(headCommit)
			if err != nil {
				return false, ""
			}
			verification := ParseCommitWithSignature(commit)
			if !verification.Verified {
				return false, ""
			}
			// need to work out merge-base
			mergeBaseCommit, _, err := gitRepo.GetMergeBase("", baseCommit, headCommit)
			if err != nil {
				return false, ""
			}
			commitList, err := commit.CommitsBeforeUntil(mergeBaseCommit)
			if err != nil {
				return false, ""
			}
			for e := commitList.Front(); e != nil; e = e.Next() {
				commit = e.Value.(*git.Commit)
				verification := ParseCommitWithSignature(commit)
				if !verification.Verified {
					return false, ""
				}
			}
		}
	}
	return true, signingKey
}
