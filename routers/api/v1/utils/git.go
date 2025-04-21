// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"errors"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/services/context"
)

type RefCommit struct {
	InputRef string
	Ref      git.RefName
	Commit   *git.Commit
	CommitID string
}

// ResolveRefCommit resolve ref to a commit if exist
func ResolveRefCommit(ctx reqctx.RequestContext, repo *repo_model.Repository, inputRef string) (_ *RefCommit, err error) {
	gitRepo, err := gitrepo.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	refCommit := RefCommit{InputRef: inputRef}
	if gitrepo.IsBranchExist(ctx, repo, inputRef) {
		refCommit.Ref = git.RefNameFromBranch(inputRef)
	} else if gitrepo.IsTagExist(ctx, repo, inputRef) {
		refCommit.Ref = git.RefNameFromTag(inputRef)
	} else if len(inputRef) == git.ObjectFormatFromName(repo.ObjectFormatName).FullLength() {
		refCommit.Ref = git.RefNameFromCommit(inputRef)
	}
	if refCommit.Ref == "" {
		return nil, git.ErrNotExist{ID: inputRef}
	}
	if refCommit.Commit, err = gitRepo.GetCommit(refCommit.Ref.String()); err != nil {
		return nil, err
	}
	refCommit.CommitID = refCommit.Commit.ID.String()
	return &refCommit, nil
}

func NewRefCommit(refName git.RefName, commit *git.Commit) *RefCommit {
	return &RefCommit{InputRef: refName.ShortName(), Ref: refName, Commit: commit, CommitID: commit.ID.String()}
}

// GetGitRefs return git references based on filter
func GetGitRefs(ctx *context.APIContext, filter string) ([]*git.Reference, string, error) {
	if ctx.Repo.GitRepo == nil {
		return nil, "", errors.New("no open git repo found in context")
	}
	if len(filter) > 0 {
		filter = "refs/" + filter
	}
	refs, err := ctx.Repo.GitRepo.GetRefsFiltered(filter)
	return refs, "GetRefsFiltered", err
}
