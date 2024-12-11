// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/util"
)

type CompareRouter struct {
	BaseOriRef    string
	BaseFullRef   git.RefName
	HeadOwnerName string
	HeadRepoName  string
	HeadOriRef    string
	HeadFullRef   git.RefName
	CaretTimes    int // ^ times after base ref
	DotTimes      int // 2(..) or 3(...)
}

func (cr *CompareRouter) IsSameRepo() bool {
	return cr.HeadOwnerName == "" && cr.HeadRepoName == ""
}

func (cr *CompareRouter) IsSameRef() bool {
	return cr.IsSameRepo() && cr.BaseOriRef == cr.HeadOriRef
}

func (cr *CompareRouter) DirectComparison() bool {
	return cr.DotTimes == 2
}

func parseBase(base string) (string, int) {
	parts := strings.SplitN(base, "^", 2)
	if len(parts) == 1 {
		return base, 0
	}
	return parts[0], len(parts[1]) + 1
}

func parseHead(head string) (string, string, string) {
	paths := strings.SplitN(head, ":", 2)
	if len(paths) == 1 {
		return "", "", paths[0]
	}
	ownerRepo := strings.SplitN(paths[0], "/", 2)
	if len(ownerRepo) == 1 {
		return "", paths[0], paths[1]
	}
	return ownerRepo[0], ownerRepo[1], paths[1]
}

func parseCompareRouter(router string) (*CompareRouter, error) {
	var basePart, headPart string
	dotTimes := 3
	parts := strings.Split(router, "...")
	if len(parts) > 2 {
		return nil, util.NewSilentWrapErrorf(util.ErrInvalidArgument, "invalid compare router: %s", router)
	}
	if len(parts) != 2 {
		parts = strings.Split(router, "..")
		if len(parts) == 1 {
			headOwnerName, headRepoName, headRef := parseHead(router)
			return &CompareRouter{
				HeadOriRef:    headRef,
				HeadOwnerName: headOwnerName,
				HeadRepoName:  headRepoName,
			}, nil
		} else if len(parts) > 2 {
			return nil, util.NewSilentWrapErrorf(util.ErrInvalidArgument, "invalid compare router: %s", router)
		}
		dotTimes = 2
	}
	basePart, headPart = parts[0], parts[1]

	baseRef, caretTimes := parseBase(basePart)
	headOwnerName, headRepoName, headRef := parseHead(headPart)

	return &CompareRouter{
		BaseOriRef:    baseRef,
		HeadOriRef:    headRef,
		HeadOwnerName: headOwnerName,
		HeadRepoName:  headRepoName,
		CaretTimes:    caretTimes,
		DotTimes:      dotTimes,
	}, nil
}

// CompareInfo represents the collected results from ParseCompareInfo
type CompareInfo struct {
	*CompareRouter
	HeadUser     *user_model.User
	HeadRepo     *repo_model.Repository
	HeadGitRepo  *git.Repository
	CompareInfo  *git.CompareInfo
	close        func()
	IsBaseCommit bool
	IsHeadCommit bool
}

// display pull related information or not
func (ci *CompareInfo) IsPull() bool {
	return ci.CaretTimes == 0 && !ci.DirectComparison() &&
		ci.BaseFullRef.IsBranch() && ci.HeadFullRef.IsBranch()
}

func (ci *CompareInfo) Close() {
	if ci.close != nil {
		ci.close()
	}
}

func detectFullRef(ctx context.Context, repoID int64, gitRepo *git.Repository, oriRef string) (git.RefName, bool, error) {
	b, err := git_model.GetBranch(ctx, repoID, oriRef)
	if err != nil {
		return "", false, err
	}
	if !b.IsDeleted {
		return git.RefNameFromBranch(oriRef), false, nil
	}

	rel, err := repo_model.GetRelease(ctx, repoID, oriRef)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		return "", false, err
	}
	if rel != nil && rel.Sha1 != "" {
		return git.RefNameFromTag(oriRef), false, nil
	}

	commitObjectID, err := gitRepo.ConvertToGitID(oriRef)
	if err != nil {
		return "", false, err
	}
	return git.RefName(commitObjectID.String()), true, nil
}

func ParseComparePathParams(ctx context.Context, pathParam string, baseRepo *repo_model.Repository, baseGitRepo *git.Repository) (*CompareInfo, error) {
	ci := &CompareInfo{}
	var err error

	if pathParam == "" {
		ci.HeadOriRef = baseRepo.DefaultBranch
	} else {
		ci.CompareRouter, err = parseCompareRouter(pathParam)
		if err != nil {
			return nil, err
		}
	}
	if ci.BaseOriRef == "" {
		ci.BaseOriRef = baseRepo.DefaultBranch
	}

	if ci.IsSameRepo() {
		ci.HeadUser = baseRepo.Owner
		ci.HeadRepo = baseRepo
		ci.HeadGitRepo = baseGitRepo
	} else {
		if ci.HeadOwnerName == baseRepo.Owner.Name {
			ci.HeadUser = baseRepo.Owner
		} else {
			ci.HeadUser, err = user_model.GetUserByName(ctx, ci.HeadOwnerName)
			if err != nil {
				return nil, err
			}
		}

		ci.HeadRepo, err = repo_model.GetRepositoryByOwnerAndName(ctx, ci.HeadOwnerName, ci.HeadRepoName)
		if err != nil {
			return nil, err
		}
		ci.HeadRepo.Owner = ci.HeadUser
		ci.HeadGitRepo, err = gitrepo.OpenRepository(ctx, ci.HeadRepo)
		if err != nil {
			return nil, err
		}
		ci.close = func() {
			if ci.HeadGitRepo != nil {
				ci.HeadGitRepo.Close()
			}
		}
	}

	ci.BaseFullRef, ci.IsBaseCommit, err = detectFullRef(ctx, baseRepo.ID, baseGitRepo, ci.BaseOriRef)
	if err != nil {
		return nil, err
	}

	ci.HeadFullRef, ci.IsHeadCommit, err = detectFullRef(ctx, ci.HeadRepo.ID, ci.HeadGitRepo, ci.HeadOriRef)
	if err != nil {
		return nil, err
	}
	return ci, nil
}
