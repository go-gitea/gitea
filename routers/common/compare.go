// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
)

type CompareRouterReq struct {
	BaseOriRef   string
	HeadOwner    string
	HeadRepoName string
	HeadOriRef   string
	CaretTimes   int // ^ times after base ref
	DotTimes     int
}

func (cr *CompareRouterReq) DirectComparison() bool {
	return cr.DotTimes == 2 || cr.DotTimes == 0
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
		return paths[0], "", paths[1]
	}
	return ownerRepo[0], ownerRepo[1], paths[1]
}

// ParseCompareRouterParam Get compare information from the router parameter.
// A full compare url is of the form:
//
// 1. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headBranch}
// 2. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headOwner}:{:headBranch}
// 3. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headOwner}/{:headRepoName}:{:headBranch}
// 4. /{:baseOwner}/{:baseRepoName}/compare/{:headBranch}
// 5. /{:baseOwner}/{:baseRepoName}/compare/{:headOwner}:{:headBranch}
// 6. /{:baseOwner}/{:baseRepoName}/compare/{:headOwner}/{:headRepoName}:{:headBranch}
//
// Here we obtain the infoPath "{:baseBranch}...[{:headOwner}/{:headRepoName}:]{:headBranch}" as ctx.PathParam("*")
// with the :baseRepo in ctx.Repo.
//
// Note: Generally :headRepoName is not provided here - we are only passed :headOwner.
//
// How do we determine the :headRepo?
//
// 1. If :headOwner is not set then the :headRepo = :baseRepo
// 2. If :headOwner is set - then look for the fork of :baseRepo owned by :headOwner
// 3. But... :baseRepo could be a fork of :headOwner's repo - so check that
// 4. Now, :baseRepo and :headRepos could be forks of the same repo - so check that
//
// format: <base branch>...[<head repo>:]<head branch>
// base<-head: master...head:feature
// same repo: master...feature
func ParseCompareRouterParam(routerParam string) (*CompareRouterReq, error) {
	if routerParam == "" {
		return &CompareRouterReq{}, nil
	}

	var basePart, headPart string
	dotTimes := 3
	parts := strings.Split(routerParam, "...")
	if len(parts) > 2 {
		return nil, util.NewInvalidArgumentErrorf("invalid compare router: %s", routerParam)
	}
	if len(parts) != 2 {
		parts = strings.Split(routerParam, "..")
		if len(parts) == 1 {
			headOwnerName, headRepoName, headRef := parseHead(routerParam)
			return &CompareRouterReq{
				HeadOriRef:   headRef,
				HeadOwner:    headOwnerName,
				HeadRepoName: headRepoName,
				DotTimes:     dotTimes,
			}, nil
		} else if len(parts) > 2 {
			return nil, util.NewInvalidArgumentErrorf("invalid compare router: %s", routerParam)
		}
		dotTimes = 2
	}
	basePart, headPart = parts[0], parts[1]

	baseRef, caretTimes := parseBase(basePart)
	headOwnerName, headRepoName, headRef := parseHead(headPart)

	return &CompareRouterReq{
		BaseOriRef:   baseRef,
		HeadOriRef:   headRef,
		HeadOwner:    headOwnerName,
		HeadRepoName: headRepoName,
		CaretTimes:   caretTimes,
		DotTimes:     dotTimes,
	}, nil
}

// maxForkTraverseLevel defines the maximum levels to traverse when searching for the head repository.
const maxForkTraverseLevel = 10

// FindHeadRepo tries to find the head repository based on the base repository and head user ID.
func FindHeadRepo(ctx context.Context, baseRepo *repo_model.Repository, headUserID int64) (*repo_model.Repository, error) {
	if baseRepo.IsFork {
		curRepo := baseRepo
		for curRepo.OwnerID != headUserID { // We assume the fork deepth is not too deep.
			if err := curRepo.GetBaseRepo(ctx); err != nil {
				return nil, err
			}
			if curRepo.BaseRepo == nil {
				return findHeadRepoFromRootBase(ctx, curRepo, headUserID, maxForkTraverseLevel)
			}
			curRepo = curRepo.BaseRepo
		}
		return curRepo, nil
	}

	return findHeadRepoFromRootBase(ctx, baseRepo, headUserID, maxForkTraverseLevel)
}

func GetHeadOwnerAndRepo(ctx context.Context, baseRepo *repo_model.Repository, compareReq *CompareRouterReq) (headOwner *user_model.User, headRepo *repo_model.Repository, err error) {
	if compareReq.HeadOwner == "" {
		if compareReq.HeadRepoName != "" { // unsupported syntax
			return nil, nil, util.ErrorWrap(util.ErrInvalidArgument, "head owner must be specified when head repo name is given")
		}

		return baseRepo.Owner, baseRepo, nil
	}

	if compareReq.HeadOwner == baseRepo.Owner.Name {
		headOwner = baseRepo.Owner
	} else {
		headOwner, err = user_model.GetUserOrOrgByName(ctx, compareReq.HeadOwner)
		if err != nil {
			return nil, nil, err
		}
	}
	if compareReq.HeadRepoName == "" {
		if headOwner.ID == baseRepo.OwnerID {
			headRepo = baseRepo
		} else {
			headRepo, err = FindHeadRepo(ctx, baseRepo, headOwner.ID)
			if err != nil {
				return nil, nil, err
			}
			if headRepo == nil {
				return nil, nil, util.ErrorWrap(util.ErrInvalidArgument, "the user %s does not have a fork of the base repository", headOwner.Name)
			}
		}
	} else {
		if compareReq.HeadOwner == baseRepo.Owner.Name && compareReq.HeadRepoName == baseRepo.Name {
			headRepo = baseRepo
		} else {
			headRepo, err = repo_model.GetRepositoryByName(ctx, headOwner.ID, compareReq.HeadRepoName)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return headOwner, headRepo, nil
}

func findHeadRepoFromRootBase(ctx context.Context, baseRepo *repo_model.Repository, headUserID int64, traverseLevel int) (*repo_model.Repository, error) {
	if traverseLevel == 0 {
		return nil, nil
	}
	// test if we are lucky
	repo, err := repo_model.GetUserFork(ctx, baseRepo.ID, headUserID)
	if err != nil {
		return nil, err
	}
	if repo != nil {
		return repo, nil
	}

	firstLevelForkedRepos, err := repo_model.GetRepositoriesByForkID(ctx, baseRepo.ID)
	if err != nil {
		return nil, err
	}
	for _, repo := range firstLevelForkedRepos {
		forked, err := findHeadRepoFromRootBase(ctx, repo, headUserID, traverseLevel-1)
		if err != nil {
			return nil, err
		}
		if forked != nil {
			return forked, nil
		}
	}
	return nil, nil
}
