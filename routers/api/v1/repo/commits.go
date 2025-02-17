// Copyright 2018 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// GetSingleCommit get a commit via sha
func GetSingleCommit(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/commits/{sha} repository repoGetSingleCommit
	// ---
	// summary: Get a single commit from a repository
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
	// - name: sha
	//   in: path
	//   description: a git ref or commit sha
	//   type: string
	//   required: true
	// - name: stat
	//   in: query
	//   description: include diff stats for every commit (disable for speedup, default 'true')
	//   type: boolean
	// - name: verification
	//   in: query
	//   description: include verification for every commit (disable for speedup, default 'true')
	//   type: boolean
	// - name: files
	//   in: query
	//   description: include a list of affected files for every commit (disable for speedup, default 'true')
	//   type: boolean
	// responses:
	//   "200":
	//     "$ref": "#/responses/Commit"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	sha := ctx.PathParam("sha")
	if !git.IsValidRefPattern(sha) {
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Sprintf("no valid ref or sha: %s", sha))
		return
	}

	getCommit(ctx, sha, convert.ParseCommitOptions(ctx))
}

func getCommit(ctx *context.APIContext, identifier string, toCommitOpts convert.ToCommitOptions) {
	commit, err := ctx.Repo.GitRepo.GetCommit(identifier)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIErrorNotFound(identifier)
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	json, err := convert.ToCommit(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, commit, nil, toCommitOpts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, json)
}

// GetAllCommits get all commits via
func GetAllCommits(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits repository repoGetAllCommits
	// ---
	// summary: Get a list of all commits from a repository
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
	// - name: sha
	//   in: query
	//   description: SHA or branch to start listing commits from (usually 'master')
	//   type: string
	// - name: path
	//   in: query
	//   description: filepath of a file/dir
	//   type: string
	// - name: stat
	//   in: query
	//   description: include diff stats for every commit (disable for speedup, default 'true')
	//   type: boolean
	// - name: verification
	//   in: query
	//   description: include verification for every commit (disable for speedup, default 'true')
	//   type: boolean
	// - name: files
	//   in: query
	//   description: include a list of affected files for every commit (disable for speedup, default 'true')
	//   type: boolean
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results (ignored if used with 'path')
	//   type: integer
	// - name: not
	//   in: query
	//   description: commits that match the given specifier will not be listed.
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommitList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/EmptyRepository"

	if ctx.Repo.Repository.IsEmpty {
		ctx.JSON(http.StatusConflict, api.APIError{
			Message: "Git Repository is empty.",
			URL:     setting.API.SwaggerURL,
		})
		return
	}

	listOptions := utils.GetListOptions(ctx)
	if listOptions.Page <= 0 {
		listOptions.Page = 1
	}

	if listOptions.PageSize > setting.Git.CommitsRangeSize {
		listOptions.PageSize = setting.Git.CommitsRangeSize
	}

	sha := ctx.FormString("sha")
	path := ctx.FormString("path")
	not := ctx.FormString("not")

	var (
		commitsCountTotal int64
		commits           []*git.Commit
		err               error
	)

	if len(path) == 0 {
		var baseCommit *git.Commit
		if len(sha) == 0 {
			// no sha supplied - use default branch
			head, err := ctx.Repo.GitRepo.GetHEADBranch()
			if err != nil {
				ctx.APIErrorInternal(err)
				return
			}

			baseCommit, err = ctx.Repo.GitRepo.GetBranchCommit(head.Name)
			if err != nil {
				ctx.APIErrorInternal(err)
				return
			}
		} else {
			// get commit specified by sha
			baseCommit, err = ctx.Repo.GitRepo.GetCommit(sha)
			if err != nil {
				ctx.NotFoundOrServerError(err)
				return
			}
		}

		// Total commit count
		commitsCountTotal, err = git.CommitsCount(ctx.Repo.GitRepo.Ctx, git.CommitsCountOptions{
			RepoPath: ctx.Repo.GitRepo.Path,
			Not:      not,
			Revision: []string{baseCommit.ID.String()},
		})
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		// Query commits
		commits, err = baseCommit.CommitsByRange(listOptions.Page, listOptions.PageSize, not)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	} else {
		if len(sha) == 0 {
			sha = ctx.Repo.Repository.DefaultBranch
		}

		commitsCountTotal, err = git.CommitsCount(ctx,
			git.CommitsCountOptions{
				RepoPath: ctx.Repo.GitRepo.Path,
				Not:      not,
				Revision: []string{sha},
				RelPath:  []string{path},
			})

		if err != nil {
			ctx.APIErrorInternal(err)
			return
		} else if commitsCountTotal == 0 {
			ctx.APIErrorNotFound("FileCommitsCount", nil)
			return
		}

		commits, err = ctx.Repo.GitRepo.CommitsByFileAndRange(
			git.CommitsByFileAndRangeOptions{
				Revision: sha,
				File:     path,
				Not:      not,
				Page:     listOptions.Page,
			})
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	pageCount := int(math.Ceil(float64(commitsCountTotal) / float64(listOptions.PageSize)))
	userCache := make(map[string]*user_model.User)
	apiCommits := make([]*api.Commit, len(commits))

	for i, commit := range commits {
		// Create json struct
		apiCommits[i], err = convert.ToCommit(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, commit, userCache, convert.ParseCommitOptions(ctx))
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	ctx.SetLinkHeader(int(commitsCountTotal), listOptions.PageSize)
	ctx.SetTotalCountHeader(commitsCountTotal)

	// kept for backwards compatibility
	ctx.RespHeader().Set("X-Page", strconv.Itoa(listOptions.Page))
	ctx.RespHeader().Set("X-PerPage", strconv.Itoa(listOptions.PageSize))
	ctx.RespHeader().Set("X-Total", strconv.FormatInt(commitsCountTotal, 10))
	ctx.RespHeader().Set("X-PageCount", strconv.Itoa(pageCount))
	ctx.RespHeader().Set("X-HasMore", strconv.FormatBool(listOptions.Page < pageCount))
	ctx.AppendAccessControlExposeHeaders("X-Page", "X-PerPage", "X-Total", "X-PageCount", "X-HasMore")

	ctx.JSON(http.StatusOK, &apiCommits)
}

// DownloadCommitDiffOrPatch render a commit's raw diff or patch
func DownloadCommitDiffOrPatch(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/commits/{sha}.{diffType} repository repoDownloadCommitDiffOrPatch
	// ---
	// summary: Get a commit's diff or patch
	// produces:
	// - text/plain
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
	// - name: sha
	//   in: path
	//   description: SHA of the commit to get
	//   type: string
	//   required: true
	// - name: diffType
	//   in: path
	//   description: whether the output is diff or patch
	//   type: string
	//   enum: [diff, patch]
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/string"
	//   "404":
	//     "$ref": "#/responses/notFound"
	sha := ctx.PathParam("sha")
	diffType := git.RawDiffType(ctx.PathParam("diffType"))

	if err := git.GetRawDiff(ctx.Repo.GitRepo, sha, diffType, ctx.Resp); err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIErrorNotFound(sha)
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
}

// GetCommitPullRequest returns the merged pull request of the commit
func GetCommitPullRequest(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{sha}/pull repository repoGetCommitPullRequest
	// ---
	// summary: Get the merged pull request of the commit
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
	// - name: sha
	//   in: path
	//   description: SHA of the commit to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullRequest"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := issues_model.GetPullRequestByMergedCommit(ctx, ctx.Repo.Repository.ID, ctx.PathParam("sha"))
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if err = pr.LoadBaseRepo(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if err = pr.LoadHeadRepo(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIPullRequest(ctx, pr, ctx.Doer))
}
