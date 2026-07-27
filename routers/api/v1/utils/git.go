// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"errors"

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

// ResolveRefCommit resolve ref to a commit if exist.
// inputRef may be a short branch/tag name, a commit ID, or a fully-qualified
// branch/tag ref (refs/heads/…, refs/tags/…) for GitHub client compatibility.
// Other refs/* names (e.g. refs/pull/, refs/for/) are rejected.
func ResolveRefCommit(ctx reqctx.RequestContext, repo *repo_model.Repository, inputRef string, minCommitIDLen ...int) (_ *RefCommit, err error) {
	gitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	refCommit := RefCommit{InputRef: inputRef}
	refName := git.RefName(inputRef)
	// Fully-qualified branch/tag only — never expose internal refs to API callers.
	if refName.IsBranch() {
		if exist, _ := git_model.IsBranchExist(ctx, repo.ID, refName.BranchName()); exist {
			refCommit.RefName = refName
		}
	} else if refName.IsTag() {
		if git.IsTagExist(ctx, repo, refName.TagName()) {
			refCommit.RefName = refName
		}
	} else if exist, _ := git_model.IsBranchExist(ctx, repo.ID, inputRef); exist {
		refCommit.RefName = git.RefNameFromBranch(inputRef)
	} else if git.IsTagExist(ctx, repo, inputRef) {
		refCommit.RefName = git.RefNameFromTag(inputRef)
	} else if git.IsStringLikelyCommitID(git.ObjectFormatFromName(repo.ObjectFormatName), inputRef, minCommitIDLen...) {
		refCommit.RefName = git.RefNameFromCommit(inputRef)
	}
	if refCommit.RefName == "" {
		return nil, git.ErrNotExist{ID: inputRef}
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
