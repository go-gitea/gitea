// Copyright 2018 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

// GetSingleCommit get a commit via
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
	//   description: the commit hash
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Commit"
	//   "404":
	//     "$ref": "#/responses/notFound"

	gitRepo, err := git.OpenRepository(ctx.Repo.Repository.RepoPath())
	if err != nil {
		ctx.ServerError("OpenRepository", err)
		return
	}
	defer gitRepo.Close()
	commit, err := gitRepo.GetCommit(ctx.Params(":sha"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommit", git.IsErrNotExist, err)
		return
	}

	json, err := toCommit(ctx, ctx.Repo.Repository, commit, nil)
	if err != nil {
		ctx.ServerError("toCommit", err)
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
	// - name: page
	//   in: query
	//   description: page number of requested commits
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
		ctx.ServerError("OpenRepository", err)
		return
	}
	defer gitRepo.Close()

	page := ctx.QueryInt("page")
	if page <= 0 {
		page = 1
	}

	sha := ctx.Query("sha")

	var baseCommit *git.Commit
	if len(sha) == 0 {
		// no sha supplied - use default branch
		head, err := gitRepo.GetHEADBranch()
		if err != nil {
			ctx.ServerError("GetHEADBranch", err)
			return
		}

		baseCommit, err = gitRepo.GetBranchCommit(head.Name)
		if err != nil {
			ctx.ServerError("GetCommit", err)
			return
		}
	} else {
		// get commit specified by sha
		baseCommit, err = gitRepo.GetCommit(sha)
		if err != nil {
			ctx.ServerError("GetCommit", err)
			return
		}
	}

	// Total commit count
	commitsCountTotal, err := baseCommit.CommitsCount()
	if err != nil {
		ctx.ServerError("GetCommitsCount", err)
		return
	}

	pageCount := int(math.Ceil(float64(commitsCountTotal) / float64(git.CommitsRangeSize)))

	// Query commits
	commits, err := baseCommit.CommitsByRange(page)
	if err != nil {
		ctx.ServerError("CommitsByRange", err)
		return
	}

	userCache := make(map[string]*models.User)

	apiCommits := make([]*api.Commit, commits.Len())

	i := 0
	for commitPointer := commits.Front(); commitPointer != nil; commitPointer = commitPointer.Next() {
		commit := commitPointer.Value.(*git.Commit)

		// Create json struct
		apiCommits[i], err = toCommit(ctx, ctx.Repo.Repository, commit, userCache)
		if err != nil {
			ctx.ServerError("toCommit", err)
			return
		}

		i++
	}

	ctx.SetLinkHeader(int(commitsCountTotal), git.CommitsRangeSize)

	ctx.Header().Set("X-Page", strconv.Itoa(page))
	ctx.Header().Set("X-PerPage", strconv.Itoa(git.CommitsRangeSize))
	ctx.Header().Set("X-Total", strconv.FormatInt(commitsCountTotal, 10))
	ctx.Header().Set("X-PageCount", strconv.Itoa(pageCount))
	ctx.Header().Set("X-HasMore", strconv.FormatBool(page < pageCount))

	ctx.JSON(http.StatusOK, &apiCommits)
}

func toCommit(ctx *context.APIContext, repo *models.Repository, commit *git.Commit, userCache map[string]*models.User) (*api.Commit, error) {

	var apiAuthor, apiCommitter *api.User

	// Retrieve author and committer information

	var cacheAuthor *models.User
	var ok bool
	if userCache == nil {
		cacheAuthor = ((*models.User)(nil))
		ok = false
	} else {
		cacheAuthor, ok = userCache[commit.Author.Email]
	}

	if ok {
		apiAuthor = cacheAuthor.APIFormat()
	} else {
		author, err := models.GetUserByEmail(commit.Author.Email)
		if err != nil && !models.IsErrUserNotExist(err) {
			return nil, err
		} else if err == nil {
			apiAuthor = author.APIFormat()
			if userCache != nil {
				userCache[commit.Author.Email] = author
			}
		}
	}

	var cacheCommitter *models.User
	if userCache == nil {
		cacheCommitter = ((*models.User)(nil))
		ok = false
	} else {
		cacheCommitter, ok = userCache[commit.Committer.Email]
	}

	if ok {
		apiCommitter = cacheCommitter.APIFormat()
	} else {
		committer, err := models.GetUserByEmail(commit.Committer.Email)
		if err != nil && !models.IsErrUserNotExist(err) {
			return nil, err
		} else if err == nil {
			apiCommitter = committer.APIFormat()
			if userCache != nil {
				userCache[commit.Committer.Email] = committer
			}
		}
	}

	// Retrieve parent(s) of the commit
	apiParents := make([]*api.CommitMeta, commit.ParentCount())
	for i := 0; i < commit.ParentCount(); i++ {
		sha, _ := commit.ParentID(i)
		apiParents[i] = &api.CommitMeta{
			URL: repo.APIURL() + "/git/commits/" + sha.String(),
			SHA: sha.String(),
		}
	}

	return &api.Commit{
		CommitMeta: &api.CommitMeta{
			URL: repo.APIURL() + "/git/commits/" + commit.ID.String(),
			SHA: commit.ID.String(),
		},
		HTMLURL: repo.HTMLURL() + "/commit/" + commit.ID.String(),
		RepoCommit: &api.RepoCommit{
			URL: repo.APIURL() + "/git/commits/" + commit.ID.String(),
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  commit.Committer.Name,
					Email: commit.Committer.Email,
				},
				Date: commit.Author.When.Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  commit.Committer.Name,
					Email: commit.Committer.Email,
				},
				Date: commit.Committer.When.Format(time.RFC3339),
			},
			Message: commit.Summary(),
			Tree: &api.CommitMeta{
				URL: repo.APIURL() + "/git/trees/" + commit.ID.String(),
				SHA: commit.ID.String(),
			},
		},
		Author:    apiAuthor,
		Committer: apiCommitter,
		Parents:   apiParents,
	}, nil
}
