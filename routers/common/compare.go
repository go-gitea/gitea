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
	pull_service "code.gitea.io/gitea/services/pull"
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

func (cr *CompareRouter) DirectComparison() bool {
	return cr.DotTimes == 2
}

func (cr *CompareRouter) CompareDots() string {
	return strings.Repeat(".", cr.DotTimes)
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

func parseCompareRouter(router string) (*CompareRouter, error) {
	var basePart, headPart string
	dotTimes := 3
	parts := strings.Split(router, "...")
	if len(parts) > 2 {
		return nil, util.NewInvalidArgumentErrorf("invalid compare router: %s", router)
	}
	if len(parts) != 2 {
		parts = strings.Split(router, "..")
		if len(parts) == 1 {
			headOwnerName, headRepoName, headRef := parseHead(router)
			return &CompareRouter{
				HeadOriRef:    headRef,
				HeadOwnerName: headOwnerName,
				HeadRepoName:  headRepoName,
				DotTimes:      dotTimes,
			}, nil
		} else if len(parts) > 2 {
			return nil, util.NewInvalidArgumentErrorf("invalid compare router: %s", router)
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
	BaseRepo     *repo_model.Repository
	HeadUser     *user_model.User
	HeadRepo     *repo_model.Repository
	HeadGitRepo  *git.Repository
	CompareInfo  *pull_service.CompareInfo
	close        func()
	IsBaseCommit bool
	IsHeadCommit bool
}

func (cr *CompareInfo) IsSameRepo() bool {
	return cr.HeadRepo.ID == cr.BaseRepo.ID
}

func (cr *CompareInfo) IsSameRef() bool {
	return cr.IsSameRepo() && cr.BaseOriRef == cr.HeadOriRef
}

// display pull related information or not
func (cr *CompareInfo) IsPull() bool {
	return cr.CaretTimes == 0 && !cr.DirectComparison() &&
		cr.BaseFullRef.IsBranch() && (cr.HeadRepo == nil || cr.HeadFullRef.IsBranch())
}

func (cr *CompareInfo) Close() {
	if cr.close != nil {
		cr.close()
	}
}

// detectFullRef detects a short name as a branch, tag or commit's full ref name and type.
// It's the same job as git.UnstableGuessRefByShortName but with a database read instead of git read.
func detectFullRef(ctx context.Context, repoID int64, gitRepo *git.Repository, oriRef string) (git.RefName, bool, error) {
	b, err := git_model.GetBranch(ctx, repoID, oriRef)
	if err != nil && !git_model.IsErrBranchNotExist(err) {
		return "", false, err
	}
	if b != nil && !b.IsDeleted {
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

func findHeadRepo(ctx context.Context, baseRepo *repo_model.Repository, headUserID int64) (*repo_model.Repository, error) {
	if baseRepo.IsFork {
		curRepo := baseRepo
		for curRepo.OwnerID != headUserID { // We assume the fork deepth is not too deep.
			if err := curRepo.GetBaseRepo(ctx); err != nil {
				return nil, err
			}
			if curRepo.BaseRepo == nil {
				return findHeadRepoFromRootBase(ctx, curRepo, headUserID, 3)
			}
			curRepo = curRepo.BaseRepo
		}
		return curRepo, nil
	}

	return findHeadRepoFromRootBase(ctx, baseRepo, headUserID, 3)
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

	firstLevelForkedRepo, err := repo_model.GetRepositoriesByForkID(ctx, baseRepo.ID)
	if err != nil {
		return nil, err
	}
	for _, repo := range firstLevelForkedRepo {
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

func getRootRepo(ctx context.Context, repo *repo_model.Repository) (*repo_model.Repository, error) {
	curRepo := repo
	for curRepo.IsFork {
		if err := curRepo.GetBaseRepo(ctx); err != nil {
			return nil, err
		}
		if curRepo.BaseRepo == nil {
			break
		}
		curRepo = curRepo.BaseRepo
	}
	return curRepo, nil
}

// ParseComparePathParams Get compare information
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
func ParseComparePathParams(ctx context.Context, pathParam string, baseRepo *repo_model.Repository, baseGitRepo *git.Repository) (*CompareInfo, error) {
	ci := &CompareInfo{BaseRepo: baseRepo}
	var err error

	if pathParam == "" {
		ci.CompareRouter = &CompareRouter{
			HeadOriRef: baseRepo.DefaultBranch,
			DotTimes:   3,
		}
	} else {
		ci.CompareRouter, err = parseCompareRouter(pathParam)
		if err != nil {
			return nil, err
		}
	}
	if ci.BaseOriRef == "" {
		ci.BaseOriRef = baseRepo.DefaultBranch
	}

	if (ci.HeadOwnerName == "" && ci.HeadRepoName == "") ||
		(ci.HeadOwnerName == baseRepo.Owner.Name && ci.HeadRepoName == baseRepo.Name) {
		ci.HeadOwnerName = baseRepo.Owner.Name
		ci.HeadRepoName = baseRepo.Name
		ci.HeadUser = baseRepo.Owner
		ci.HeadRepo = baseRepo
		ci.HeadGitRepo = baseGitRepo
	} else {
		if ci.HeadOwnerName == baseRepo.Owner.Name {
			ci.HeadUser = baseRepo.Owner
			if ci.HeadRepoName == "" {
				ci.HeadRepoName = baseRepo.Name
				ci.HeadRepo = baseRepo
			}
		} else {
			ci.HeadUser, err = user_model.GetUserByName(ctx, ci.HeadOwnerName)
			if err != nil {
				return nil, err
			}
		}

		if ci.HeadRepo == nil {
			if ci.HeadRepoName != "" {
				ci.HeadRepo, err = repo_model.GetRepositoryByOwnerAndName(ctx, ci.HeadOwnerName, ci.HeadRepoName)
			} else {
				ci.HeadRepo, err = findHeadRepo(ctx, baseRepo, ci.HeadUser.ID)
			}
			if err != nil {
				return nil, err
			}
		}
		if ci.HeadRepo != nil {
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
	}

	ci.BaseFullRef, ci.IsBaseCommit, err = detectFullRef(ctx, baseRepo.ID, baseGitRepo, ci.BaseOriRef)
	if err != nil {
		ci.Close()
		return nil, err
	}

	if ci.HeadRepo != nil {
		ci.HeadFullRef, ci.IsHeadCommit, err = detectFullRef(ctx, ci.HeadRepo.ID, ci.HeadGitRepo, ci.HeadOriRef)
		if err != nil {
			ci.Close()
			return nil, err
		}
	}
	return ci, nil
}

func (cr *CompareInfo) LoadRootRepoAndOwnForkRepo(ctx context.Context, baseRepo *repo_model.Repository, doer *user_model.User) (*repo_model.Repository, *repo_model.Repository, error) {
	// find root repo
	var rootRepo *repo_model.Repository
	var err error
	if !baseRepo.IsFork {
		rootRepo = baseRepo
	} else {
		if !cr.HeadRepo.IsFork {
			rootRepo = cr.HeadRepo
		} else {
			rootRepo, err = getRootRepo(ctx, baseRepo)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// find ownfork repo
	var ownForkRepo *repo_model.Repository
	if doer != nil && cr.HeadRepo.OwnerID != doer.ID && baseRepo.OwnerID != doer.ID {
		ownForkRepo, err = findHeadRepo(ctx, baseRepo, doer.ID)
		if err != nil {
			return nil, nil, err
		}
	}

	return rootRepo, ownForkRepo, nil
}
