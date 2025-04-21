// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/services/context"
	"errors"
)

type RefCommit struct {
	InputRef string
	Ref      git.RefName
	Commit   *git.Commit
	CommitID string
}

// ResolveRefCommit resolve ref to a commit if exist
func ResolveRefCommit(ctx *context.APIContext, inputRef string) (_ *RefCommit, err error) {
	var refCommit RefCommit
	if gitrepo.IsBranchExist(ctx, ctx.Repo.Repository, inputRef) {
		refCommit.Ref = git.RefNameFromBranch(inputRef)
	} else if gitrepo.IsTagExist(ctx, ctx.Repo.Repository, inputRef) {
		refCommit.Ref = git.RefNameFromTag(inputRef)
	} else if len(inputRef) == ctx.Repo.GetObjectFormat().FullLength() {
		refCommit.Ref = git.RefNameFromCommit(inputRef)
	}
	if refCommit.Ref == "" {
		return nil, git.ErrNotExist{ID: inputRef}
	}
	if refCommit.Commit, err = ctx.Repo.GitRepo.GetCommit(refCommit.Ref.String()); err != nil {
		return nil, err
	}
	refCommit.InputRef = inputRef
	refCommit.CommitID = refCommit.Commit.ID.String()
	return &refCommit, nil
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
