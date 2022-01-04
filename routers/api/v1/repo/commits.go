// Copyright 2018 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/validation"
	"code.gitea.io/gitea/routers/api/v1/utils"
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/Commit"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	sha := ctx.Params(":sha")
	if (validation.GitRefNamePatternInvalid.MatchString(sha) || !validation.CheckGitRefAdditionalRulesValid(sha)) && !git.SHAPattern.MatchString(sha) {
		ctx.Error(http.StatusUnprocessableEntity, "no valid ref or sha", fmt.Sprintf("no valid ref or sha: %s", sha))
		return
	}
	getCommit(ctx, sha)
}

func getCommit(ctx *context.APIContext, identifier string) {
	gitRepo, err := git.OpenRepository(ctx.Repo.Repository.RepoPath())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
		return
	}
	defer gitRepo.Close()
	commit, err := gitRepo.GetCommit(identifier)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(identifier)
			return
		}
		ctx.Error(http.StatusInternalServerError, "gitRepo.GetCommit", err)
		return
	}

	json, err := convert.ToCommit(ctx.Repo.Repository, commit, nil)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "toCommit", err)
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results (ignored if used with 'path')
	//   type: integer
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

	gitRepo, err := git.OpenRepository(ctx.Repo.Repository.RepoPath())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
		return
	}
	defer gitRepo.Close()

	listOptions := utils.GetListOptions(ctx)
	if listOptions.Page <= 0 {
		listOptions.Page = 1
	}

	if listOptions.PageSize > setting.Git.CommitsRangeSize {
		listOptions.PageSize = setting.Git.CommitsRangeSize
	}

	sha := ctx.FormString("sha")
	path := ctx.FormString("path")

	var (
		commitsCountTotal int64
		commits           []*git.Commit
	)

	if len(path) == 0 {
		var baseCommit *git.Commit
		if len(sha) == 0 {
			// no sha supplied - use default branch
			head, err := gitRepo.GetHEADBranch()
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetHEADBranch", err)
				return
			}

			baseCommit, err = gitRepo.GetBranchCommit(head.Name)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetCommit", err)
				return
			}
		} else {
			// get commit specified by sha
			baseCommit, err = gitRepo.GetCommit(sha)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetCommit", err)
				return
			}
		}

		// Total commit count
		commitsCountTotal, err = baseCommit.CommitsCount()
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetCommitsCount", err)
			return
		}

		// Query commits
		commits, err = baseCommit.CommitsByRange(listOptions.Page, listOptions.PageSize)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "CommitsByRange", err)
			return
		}
	} else {
		if len(sha) == 0 {
			sha = ctx.Repo.Repository.DefaultBranch
		}

		commitsCountTotal, err = gitRepo.FileCommitsCount(sha, path)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "FileCommitsCount", err)
			return
		} else if commitsCountTotal == 0 {
			ctx.NotFound("FileCommitsCount", nil)
			return
		}

		commits, err = gitRepo.CommitsByFileAndRange(sha, path, listOptions.Page)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "CommitsByFileAndRange", err)
			return
		}
	}

	pageCount := int(math.Ceil(float64(commitsCountTotal) / float64(listOptions.PageSize)))

	userCache := make(map[string]*user_model.User)

	apiCommits := make([]*api.Commit, len(commits))
	for i, commit := range commits {
		// Create json struct
		apiCommits[i], err = convert.ToCommit(ctx.Repo.Repository, commit, userCache)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "toCommit", err)
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
	repoPath := repo_model.RepoPath(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
	if err := git.GetRawDiff(
		repoPath,
		ctx.Params(":sha"),
		git.RawDiffType(ctx.Params(":diffType")),
		ctx.Resp,
	); err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(ctx.Params(":sha"))
			return
		}
		ctx.Error(http.StatusInternalServerError, "DownloadCommitDiffOrPatch", err)
		return
	}
}
