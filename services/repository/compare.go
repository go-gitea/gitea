// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/gitdiff"
)

// CompareInfo represents the collected results from ParseCompareInfo
type CompareInfo struct {
	HeadUser         *user_model.User
	baseRepo         *repo_model.Repository
	baseGitRepo      *git.Repository
	HeadRepo         *repo_model.Repository
	HeadGitRepo      *git.Repository
	CompareInfo      *git.CompareInfo
	BaseBranch       string
	HeadBranch       string
	DirectComparison bool
	isSameRepo       bool

	BeforeCommit *git.Commit
}

func (ci *CompareInfo) IsSameRepo() bool {
	return ci.isSameRepo
}

type ErrCompareNotFound struct {
	Func string
}

func (e ErrCompareNotFound) Error() string {
	return "compare info is not found: " + e.Func
}

func IsErrCompareNotFound(e error) bool {
	_, ok := e.(ErrCompareNotFound)
	return ok
}

func getBranchesAndTagsForRepo(ctx context.Context, repo *repo_model.Repository) (branches, tags []string, err error) {
	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		return nil, nil, err
	}
	defer gitRepo.Close()

	branches, err = git_model.FindBranchNames(ctx, git_model.FindBranchOptions{
		RepoID: repo.ID,
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		IsDeletedBranch: util.OptionalBoolFalse,
	})
	if err != nil {
		return nil, nil, err
	}
	tags, err = gitRepo.GetTags(0, 0)
	if err != nil {
		return nil, nil, err
	}
	return branches, tags, nil
}

// ParseCompareInfo parse compare info between two commit for preparing comparing references
func ParseCompareInfo(ctx *gitea_context.Base, baseRepo *repo_model.Repository, baseGitRepo *git.Repository, doer *user_model.User) (*CompareInfo, error) {
	ci := &CompareInfo{}

	fileOnly := ctx.FormBool("file-only")

	// Get compared branches information
	// A full compare url is of the form:
	//
	// 1. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headBranch}
	// 2. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headOwner}:{:headBranch}
	// 3. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headOwner}/{:headRepoName}:{:headBranch}
	// 4. /{:baseOwner}/{:baseRepoName}/compare/{:headBranch}
	// 5. /{:baseOwner}/{:baseRepoName}/compare/{:headOwner}:{:headBranch}
	// 6. /{:baseOwner}/{:baseRepoName}/compare/{:headOwner}/{:headRepoName}:{:headBranch}
	//
	// Here we obtain the infoPath "{:baseBranch}...[{:headOwner}/{:headRepoName}:]{:headBranch}" as ctx.Params("*")
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

	var (
		infoPath string
		err      error
	)

	infoPath = ctx.Params("*")
	var infos []string
	if infoPath == "" {
		infos = []string{baseRepo.DefaultBranch, baseRepo.DefaultBranch}
	} else {
		infos = strings.SplitN(infoPath, "...", 2)
		if len(infos) != 2 {
			if infos = strings.SplitN(infoPath, "..", 2); len(infos) == 2 {
				ci.DirectComparison = true
				ctx.Data["PageIsComparePull"] = false
			} else {
				infos = []string{baseRepo.DefaultBranch, infoPath}
			}
		}
	}

	ctx.Data["BaseName"] = baseRepo.OwnerName
	ci.BaseBranch = infos[0]
	ctx.Data["BaseBranch"] = ci.BaseBranch

	// If there is no head repository, it means compare between same repository.
	headInfos := strings.Split(infos[1], ":")
	if len(headInfos) == 1 {
		ci.isSameRepo = true
		ci.HeadUser = baseRepo.Owner
		ci.HeadBranch = headInfos[0]

	} else if len(headInfos) == 2 {
		headInfosSplit := strings.Split(headInfos[0], "/")
		if len(headInfosSplit) == 1 {
			ci.HeadUser, err = user_model.GetUserByName(ctx, headInfos[0])
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					return nil, ErrCompareNotFound{Func: "GetUserByName"}
				}
				return nil, err
			}
			ci.HeadBranch = headInfos[1]
			ci.isSameRepo = ci.HeadUser.ID == baseRepo.OwnerID
			if ci.isSameRepo {
				ci.HeadRepo = baseRepo
			}
		} else {
			ci.HeadRepo, err = repo_model.GetRepositoryByOwnerAndName(ctx, headInfosSplit[0], headInfosSplit[1])
			if err != nil {
				if repo_model.IsErrRepoNotExist(err) {
					return nil, ErrCompareNotFound{Func: "GetRepositoryByOwnerAndName"}
				}
				return nil, err
			}
			if err := ci.HeadRepo.LoadOwner(ctx); err != nil {
				if user_model.IsErrUserNotExist(err) {
					return nil, ErrCompareNotFound{Func: "GetUserByName"}
				}
				return nil, err
			}
			ci.HeadBranch = headInfos[1]
			ci.HeadUser = ci.HeadRepo.Owner
			ci.isSameRepo = ci.HeadRepo.ID == baseRepo.ID
		}
	} else {
		return nil, ErrCompareNotFound{Func: "CompareAndPullRequest"}
	}
	ctx.Data["HeadUser"] = ci.HeadUser
	ctx.Data["HeadBranch"] = ci.HeadBranch

	// Check if base branch is valid.
	baseIsCommit := baseGitRepo.IsCommitExist(ci.BaseBranch)
	baseIsBranch := baseGitRepo.IsBranchExist(ci.BaseBranch)
	baseIsTag := baseGitRepo.IsTagExist(ci.BaseBranch)
	if !baseIsCommit && !baseIsBranch && !baseIsTag {
		// Check if baseBranch is short sha commit hash
		if baseCommit, _ := baseGitRepo.GetCommit(ci.BaseBranch); baseCommit != nil {
			ci.BaseBranch = baseCommit.ID.String()
			ctx.Data["BaseBranch"] = ci.BaseBranch
			baseIsCommit = true
		} else if ci.BaseBranch == git.EmptySHA {
			return ci, nil
		} else {
			return ci, ErrCompareNotFound{Func: "IsBaseBranchEmptySHA"}
		}
	}
	ctx.Data["BaseIsCommit"] = baseIsCommit
	ctx.Data["BaseIsBranch"] = baseIsBranch
	ctx.Data["BaseIsTag"] = baseIsTag
	ctx.Data["IsPull"] = true

	// Now we have the repository that represents the base

	// The current base and head repositories and branches may not
	// actually be the intended branches that the user wants to
	// create a pull-request from - but also determining the head
	// repo is difficult.

	// We will want therefore to offer a few repositories to set as
	// our base and head

	// 1. First if the baseRepo is a fork get the "RootRepo" it was
	// forked from
	var rootRepo *repo_model.Repository
	if baseRepo.IsFork {
		err = baseRepo.GetBaseRepo(ctx)
		if err != nil {
			if !repo_model.IsErrRepoNotExist(err) {
				return nil, err
			}
		} else {
			rootRepo = baseRepo.BaseRepo
		}
	}

	// 2. Now if the current user is not the owner of the baseRepo,
	// check if they have a fork of the base repo and offer that as
	// "OwnForkRepo"
	var ownForkRepo *repo_model.Repository
	if doer != nil && baseRepo.OwnerID != doer.ID {
		repo := repo_model.GetForkedRepo(doer.ID, baseRepo.ID)
		if repo != nil {
			ownForkRepo = repo
			ctx.Data["OwnForkRepo"] = ownForkRepo
		}
	}

	has := ci.HeadRepo != nil
	// 3. If the base is a forked from "RootRepo" and the owner of
	// the "RootRepo" is the :headUser - set headRepo to that
	if !has && rootRepo != nil && rootRepo.OwnerID == ci.HeadUser.ID {
		ci.HeadRepo = rootRepo
		has = true
	}

	// 4. If the doer has their own fork of the baseRepo and the headUser is the doer
	// set the headRepo to the ownFork
	if !has && ownForkRepo != nil && ownForkRepo.OwnerID == ci.HeadUser.ID {
		ci.HeadRepo = ownForkRepo
		has = true
	}

	// 5. If the headOwner has a fork of the baseRepo - use that
	if !has {
		ci.HeadRepo = repo_model.GetForkedRepo(ci.HeadUser.ID, baseRepo.ID)
		has = ci.HeadRepo != nil
	}

	// 6. If the baseRepo is a fork and the headUser has a fork of that use that
	if !has && baseRepo.IsFork {
		ci.HeadRepo = repo_model.GetForkedRepo(ci.HeadUser.ID, baseRepo.ForkID)
		has = ci.HeadRepo != nil
	}

	// 7. Otherwise if we're not the same repo and haven't found a repo give up
	if !ci.isSameRepo && !has {
		ctx.Data["PageIsComparePull"] = false
	}

	// 8. Finally open the git repo
	if ci.isSameRepo {
		ci.HeadRepo = baseRepo
		ci.HeadGitRepo = baseGitRepo
	} else if has {
		ci.HeadGitRepo, err = git.OpenRepository(ctx, ci.HeadRepo.RepoPath())
		if err != nil {
			return nil, err
		}
		defer ci.HeadGitRepo.Close()
	}

	ctx.Data["HeadRepo"] = ci.HeadRepo
	ctx.Data["BaseCompareRepo"] = baseRepo

	// Now we need to assert that the doer has permission to read
	// the baseRepo's code and pulls
	// (NOT headRepo's)
	permBase, err := access_model.GetUserRepoPermission(ctx, baseRepo, doer)
	if err != nil {
		return nil, err
	}
	if !permBase.CanRead(unit.TypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				doer,
				baseRepo,
				permBase)
		}
		return nil, ErrCompareNotFound{Func: "ParseCompareInfo"}
	}

	// If we're not merging from the same repo:
	if !ci.isSameRepo {
		// Assert doer has permission to read headRepo's codes
		permHead, err := access_model.GetUserRepoPermission(ctx, ci.HeadRepo, doer)
		if err != nil {
			return nil, err
		}
		if !permHead.CanRead(unit.TypeCode) {
			if log.IsTrace() {
				log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in headRepo has Permissions: %-+v",
					doer,
					ci.HeadRepo,
					permHead)
			}
			return nil, ErrCompareNotFound{Func: "ParseCompareInfo"}
		}
		ctx.Data["CanWriteToHeadRepo"] = permHead.CanWrite(unit.TypeCode)
	}

	// If we have a rootRepo and it's different from:
	// 1. the computed base
	// 2. the computed head
	// then get the branches of it
	if rootRepo != nil &&
		rootRepo.ID != ci.HeadRepo.ID &&
		rootRepo.ID != baseRepo.ID {
		canRead := access_model.CheckRepoUnitUser(ctx, rootRepo, doer, unit.TypeCode)
		if canRead && rootRepo.AllowsPulls() {
			ctx.Data["RootRepo"] = rootRepo
			if !fileOnly {
				branches, tags, err := getBranchesAndTagsForRepo(ctx, rootRepo)
				if err != nil {
					return nil, err
				}

				ctx.Data["RootRepoBranches"] = branches
				ctx.Data["RootRepoTags"] = tags
			}
		}
	}

	// If we have a ownForkRepo and it's different from:
	// 1. The computed base
	// 2. The computed head
	// 3. The rootRepo (if we have one)
	// then get the branches from it.
	if ownForkRepo != nil &&
		ownForkRepo.ID != ci.HeadRepo.ID &&
		ownForkRepo.ID != baseRepo.ID &&
		(rootRepo == nil || ownForkRepo.ID != rootRepo.ID) {
		canRead := access_model.CheckRepoUnitUser(ctx, ownForkRepo, doer, unit.TypeCode)
		if canRead {
			ctx.Data["OwnForkRepo"] = ownForkRepo
			if !fileOnly {
				branches, tags, err := getBranchesAndTagsForRepo(ctx, ownForkRepo)
				if err != nil {
					return nil, err
				}
				ctx.Data["OwnForkRepoBranches"] = branches
				ctx.Data["OwnForkRepoTags"] = tags
			}
		}
	}

	// Check if head branch is valid.
	headIsCommit := ci.HeadGitRepo.IsCommitExist(ci.HeadBranch)
	headIsBranch := ci.HeadGitRepo.IsBranchExist(ci.HeadBranch)
	headIsTag := ci.HeadGitRepo.IsTagExist(ci.HeadBranch)
	if !headIsCommit && !headIsBranch && !headIsTag {
		// Check if headBranch is short sha commit hash
		if headCommit, _ := ci.HeadGitRepo.GetCommit(ci.HeadBranch); headCommit != nil {
			ci.HeadBranch = headCommit.ID.String()
			ctx.Data["HeadBranch"] = ci.HeadBranch
			headIsCommit = true
		} else {
			return nil, ErrCompareNotFound{Func: "IsRefExist"}
		}
	}
	ctx.Data["HeadIsCommit"] = headIsCommit
	ctx.Data["HeadIsBranch"] = headIsBranch
	ctx.Data["HeadIsTag"] = headIsTag

	// Treat as pull request if both references are branches
	if ctx.Data["PageIsComparePull"] == nil {
		ctx.Data["PageIsComparePull"] = headIsBranch && baseIsBranch
	}

	if ctx.Data["PageIsComparePull"] == true && !permBase.CanReadIssuesOrPulls(true) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot create/read pull requests in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				doer,
				baseRepo,
				permBase)
		}
		return nil, ErrCompareNotFound{Func: "ParseCompareInfo"}
	}

	baseBranchRef := ci.BaseBranch
	if baseIsBranch {
		baseBranchRef = git.BranchPrefix + ci.BaseBranch
	} else if baseIsTag {
		baseBranchRef = git.TagPrefix + ci.BaseBranch
	}
	headBranchRef := ci.HeadBranch
	if headIsBranch {
		headBranchRef = git.BranchPrefix + ci.HeadBranch
	} else if headIsTag {
		headBranchRef = git.TagPrefix + ci.HeadBranch
	}

	ci.CompareInfo, err = ci.HeadGitRepo.GetCompareInfo(baseRepo.RepoPath(), baseBranchRef, headBranchRef, ci.DirectComparison, fileOnly)
	if err != nil {
		return nil, err
	}
	if ci.DirectComparison {
		ctx.Data["BeforeCommitID"] = ci.CompareInfo.BaseCommitID
	} else {
		ctx.Data["BeforeCommitID"] = ci.CompareInfo.MergeBase
	}

	ci.baseRepo = baseRepo
	ci.baseGitRepo = baseGitRepo

	return ci, nil
}

// LoadCompareDiff load compare diff data
func (ci *CompareInfo) LoadCompareDiff(
	whitespaceBehavior git.TrustedCmdArgs,
	files []string,
	skipTo string,
) (*gitdiff.Diff, bool, error) {
	headCommitID := ci.CompareInfo.HeadCommitID

	beforeCommitID := ci.CompareInfo.MergeBase
	if ci.DirectComparison {
		beforeCommitID = ci.CompareInfo.BaseCommitID
	}

	var err error
	ci.BeforeCommit, err = ci.baseGitRepo.GetCommit(beforeCommitID)
	if err != nil {
		return nil, false, err
	}

	maxLines, maxFiles := setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffFiles
	if len(files) == 2 || len(files) == 1 {
		maxLines, maxFiles = -1, -1
	}

	diff, err := gitdiff.GetDiff(ci.HeadGitRepo,
		&gitdiff.DiffOptions{
			BeforeCommitID:     beforeCommitID,
			AfterCommitID:      headCommitID,
			SkipTo:             skipTo,
			MaxLines:           maxLines,
			MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
			MaxFiles:           maxFiles,
			WhitespaceBehavior: whitespaceBehavior,
			DirectComparison:   ci.DirectComparison,
		}, files...)
	if err != nil {
		return nil, false, err
	}

	return diff, false, nil
}
