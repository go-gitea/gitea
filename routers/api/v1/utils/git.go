// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"errors"
	"strings"

	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/reqctx"
	"gitea.dev/services/context"
)

type RefCommit struct {
	InputRef string
	RefName  git.RefName
	Commit   *git.Commit
	CommitID string
}

// ResolveRefCommit resolve ref to a commit if it exists.
// inputRef may be a short branch/tag name, a commit ID, or a fully-qualified
// git ref (e.g. refs/heads/main, refs/tags/v1.0) for GitHub client compatibility.
func ResolveRefCommit(ctx reqctx.RequestContext, repo *repo_model.Repository, inputRef string, minCommitIDLen ...int) (_ *RefCommit, err error) {
	refCommit := RefCommit{InputRef: inputRef}

	var testRefBranch, testRefTag git.RefName
	if strings.HasPrefix(inputRef, "refs/") {
		// Fully-qualified refs first so clients can pass unambiguous names.
		testRef := git.RefName(inputRef)
		if testRef.IsBranch() {
			testRefBranch = testRef
		} else if testRef.IsTag() {
			testRefTag = testRef
		}
	} else {
		testRefBranch, testRefTag = git.RefNameFromBranch(inputRef), git.RefNameFromTag(inputRef)
	}

	if testRefBranch != "" {
		if exist, _ := git_model.IsBranchExist(ctx, repo.ID, testRefBranch.ShortName()); exist {
			refCommit.RefName = testRefBranch
		}
	}
	if testRefTag != "" {
		// TODO: use git model instead of git command in the future? Model seems to be faster.
		if git.IsTagExist(ctx, repo, testRefTag.ShortName()) {
			refCommit.RefName = testRefTag
		}
	}
	if refCommit.RefName == "" {
		if git.IsStringLikelyCommitID(git.ObjectFormatFromName(repo.ObjectFormatName), inputRef, minCommitIDLen...) {
			refCommit.RefName = git.RefNameFromCommit(inputRef)
		}
	}

	if refCommit.RefName == "" {
		return nil, git.ErrNotExist{ID: inputRef}
	}

	gitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	if refCommit.Commit, err = gitRepo.GetCommit(ctx, refCommit.RefName.String()); err != nil {
		return nil, err
	}
	refCommit.CommitID = refCommit.Commit.ID.String()
	return &refCommit, nil
}

func NewRefCommit(refName git.RefName, commit *git.Commit) *RefCommit {
	return &RefCommit{InputRef: refName.ShortName(), RefName: refName, Commit: commit, CommitID: commit.ID.String()}
}

// GetGitRefs return git references based on filter
func GetGitRefs(ctx *context.APIContext, filter string) ([]*git.Reference, string, error) {
	if ctx.Repo.GitRepo == nil {
		return nil, "", errors.New("no open git repo found in context")
	}
	if len(filter) > 0 {
		filter = "refs/" + filter
	}
	refs, err := ctx.Repo.GitRepo.GetRefsFiltered(ctx, filter)
	return refs, "GetRefsFiltered", err
}
