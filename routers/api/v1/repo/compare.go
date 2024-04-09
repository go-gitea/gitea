// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"

	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// ParseCompareInfo parse compare info between two commit for preparing comparing references
func ParseCompareInfo(ctx *context.APIContext) *common.CompareInfo {
	baseRepo := ctx.Repo.Repository
	ci := &common.CompareInfo{}

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
		isSameRepo bool
		infoPath   string
		err        error
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
			} else {
				infos = []string{baseRepo.DefaultBranch, infoPath}
			}
		}
	}

	ci.BaseBranch = infos[0]

	// If there is no head repository, it means compare between same repository.
	headInfos := strings.Split(infos[1], ":")
	if len(headInfos) == 1 {
		isSameRepo = true
		ci.HeadUser = ctx.Repo.Owner
		ci.HeadBranch = headInfos[0]
	} else if len(headInfos) == 2 {
		headInfosSplit := strings.Split(headInfos[0], "/")
		if len(headInfosSplit) == 1 {
			ci.HeadUser, err = user_model.GetUserByName(ctx, headInfos[0])
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					ctx.NotFound("GetUserByName", nil)
				} else {
					ctx.ServerError("GetUserByName", err)
				}
				return nil
			}
			ci.HeadBranch = headInfos[1]
			isSameRepo = ci.HeadUser.ID == ctx.Repo.Owner.ID
			if isSameRepo {
				ci.HeadRepo = baseRepo
			}
		} else {
			ci.HeadRepo, err = repo_model.GetRepositoryByOwnerAndName(ctx, headInfosSplit[0], headInfosSplit[1])
			if err != nil {
				if repo_model.IsErrRepoNotExist(err) {
					ctx.NotFound("GetRepositoryByOwnerAndName", nil)
				} else {
					ctx.ServerError("GetRepositoryByOwnerAndName", err)
				}
				return nil
			}
			if err := ci.HeadRepo.LoadOwner(ctx); err != nil {
				if user_model.IsErrUserNotExist(err) {
					ctx.NotFound("GetUserByName", nil)
				} else {
					ctx.ServerError("GetUserByName", err)
				}
				return nil
			}
			ci.HeadBranch = headInfos[1]
			ci.HeadUser = ci.HeadRepo.Owner
			isSameRepo = ci.HeadRepo.ID == ctx.Repo.Repository.ID
		}
	} else {
		ctx.NotFound("CompareAndPullRequest", nil)
		return nil
	}
	ctx.Repo.PullRequest.SameRepo = isSameRepo

	// Check if base branch is valid.
	baseIsCommit := ctx.Repo.GitRepo.IsCommitExist(ci.BaseBranch)
	baseIsBranch := ctx.Repo.GitRepo.IsBranchExist(ci.BaseBranch)
	baseIsTag := ctx.Repo.GitRepo.IsTagExist(ci.BaseBranch)

	if !baseIsCommit && !baseIsBranch && !baseIsTag {
		// Check if baseBranch is short sha commit hash
		if baseCommit, _ := ctx.Repo.GitRepo.GetCommit(ci.BaseBranch); baseCommit != nil {
			ci.BaseBranch = baseCommit.ID.String()
		} else if ci.BaseBranch == ctx.Repo.GetObjectFormat().EmptyObjectID().String() {
			if isSameRepo {
				ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ci.HeadBranch))
			} else {
				ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ci.HeadRepo.FullName()) + ":" + util.PathEscapeSegments(ci.HeadBranch))
			}
			return nil
		} else {
			ctx.NotFound("IsRefExist", nil)
			return nil
		}
	}

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
				ctx.ServerError("Unable to find root repo", err)
				return nil
			}
		} else {
			rootRepo = baseRepo.BaseRepo
		}
	}

	// 2. Now if the current user is not the owner of the baseRepo,
	// check if they have a fork of the base repo and offer that as
	// "OwnForkRepo"
	var ownForkRepo *repo_model.Repository
	if ctx.Doer != nil && baseRepo.OwnerID != ctx.Doer.ID {
		repo := repo_model.GetForkedRepo(ctx, ctx.Doer.ID, baseRepo.ID)
		if repo != nil {
			ownForkRepo = repo
		}
	}

	has := ci.HeadRepo != nil
	// 3. If the base is a forked from "RootRepo" and the owner of
	// the "RootRepo" is the :headUser - set headRepo to that
	if !has && rootRepo != nil && rootRepo.OwnerID == ci.HeadUser.ID {
		ci.HeadRepo = rootRepo
		has = true
	}

	// 4. If the ctx.Doer has their own fork of the baseRepo and the headUser is the ctx.Doer
	// set the headRepo to the ownFork
	if !has && ownForkRepo != nil && ownForkRepo.OwnerID == ci.HeadUser.ID {
		ci.HeadRepo = ownForkRepo
		has = true
	}

	// 5. If the headOwner has a fork of the baseRepo - use that
	if !has {
		ci.HeadRepo = repo_model.GetForkedRepo(ctx, ci.HeadUser.ID, baseRepo.ID)
		has = ci.HeadRepo != nil
	}

	// 6. If the baseRepo is a fork and the headUser has a fork of that use that
	if !has && baseRepo.IsFork {
		ci.HeadRepo = repo_model.GetForkedRepo(ctx, ci.HeadUser.ID, baseRepo.ForkID)
		has = ci.HeadRepo != nil
	}

	// 7. Finally open the git repo
	if isSameRepo {
		ci.HeadRepo = ctx.Repo.Repository
		ci.HeadGitRepo = ctx.Repo.GitRepo
	} else if has {
		ci.HeadGitRepo, err = gitrepo.OpenRepository(ctx, ci.HeadRepo)
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return nil
		}
		defer ci.HeadGitRepo.Close()
	} else {
		ctx.NotFound("ParseCompareInfo", nil)
		return nil
	}

	// Now we need to assert that the ctx.Doer has permission to read
	// the baseRepo's code and pulls
	// (NOT headRepo's)
	permBase, err := access_model.GetUserRepoPermission(ctx, baseRepo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil
	}
	if !permBase.CanRead(unit.TypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.Doer,
				baseRepo,
				permBase)
		}
		ctx.NotFound("ParseCompareInfo", nil)
		return nil
	}

	// If we're not merging from the same repo:
	if !isSameRepo {
		// Assert ctx.Doer has permission to read headRepo's codes
		permHead, err := access_model.GetUserRepoPermission(ctx, ci.HeadRepo, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserRepoPermission", err)
			return nil
		}
		if !permHead.CanRead(unit.TypeCode) {
			if log.IsTrace() {
				log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in headRepo has Permissions: %-+v",
					ctx.Doer,
					ci.HeadRepo,
					permHead)
			}
			ctx.NotFound("ParseCompareInfo", nil)
			return nil
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
		} else {
			ctx.NotFound("IsRefExist", nil)
			return nil
		}
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
		ctx.ServerError("GetCompareInfo", err)
		return nil
	}

	return ci
}

// CompareDiff compare two branches or commits
func CompareDiff(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/compare/{basehead} Get commit comparison information
	// ---
	// summary: Get commit comparison information
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: basehead
	//   in: path
	//   description: compare two branches or commits
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Compare"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if ctx.Repo.GitRepo == nil {
		gitRepo, err := gitrepo.OpenRepository(ctx, ctx.Repo.Repository)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
			return
		}
		ctx.Repo.GitRepo = gitRepo
		defer gitRepo.Close()
	}

	ci := ParseCompareInfo(ctx)
	defer func() {
		if ci != nil && ci.HeadGitRepo != nil {
			ci.HeadGitRepo.Close()
		}
	}()
	if ctx.Written() {
		return
	}

	apiCommits := make([]*api.Commit, 0, len(ci.CompareInfo.Commits))
	userCache := make(map[string]*user_model.User)
	for i := 0; i < len(ci.CompareInfo.Commits); i++ {
		apiCommit, err := convert.ToCommit(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, ci.CompareInfo.Commits[i], userCache,
			convert.ToCommitOptions{
				Stat:         true,
				Verification: ctx.FormBool("verification"),
				Files:        ctx.FormBool("files"),
			})
		if err != nil {
			ctx.ServerError("toCommit", err)
			return
		}
		apiCommits = append(apiCommits, apiCommit)
	}

	ctx.JSON(http.StatusOK, &api.Compare{
		TotalCommits: len(ci.CompareInfo.Commits),
		Commits:      apiCommits,
	})
}
