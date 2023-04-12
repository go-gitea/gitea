package repo

import (
	"fmt"
	"net/http"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
	files_service "code.gitea.io/gitea/services/repository/files"
)

// CreateCheckRun Create a new check run
func CreateCheckRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/check-runs checkRuns checkRunsCreate
	// ---
	// summary: Create a new check run
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateCheckRunOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/CheckRun"
	//   "400":
	//     "$ref": "#/responses/error"

	form := web.GetForm(ctx).(*api.CreateCheckRunOptions)

	checkRun, err := files_service.CreateCheckRun(ctx, ctx.Repo.Repository, ctx.Doer, form)
	if err != nil {
		if git_model.IsErrCheckRunExist(err) || git_model.IsErrUnVaildCheckRunOptions(err) {
			ctx.Error(http.StatusBadRequest, "CreateCheckRun", err)
			return
		}

		ctx.Error(http.StatusInternalServerError, "CreateCheckRun", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToChekckRun(ctx, checkRun))
}

// GetCheckRun Get a check run
func GetCheckRun(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/check-runs/{check_run_id} checkRuns checkRunsGet
	// ---
	// summary: Get a check run
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
	// - name: check_run_id
	//   in: path
	//   description: id of the check run
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/CheckRun"
	//   "404":
	//     "$ref": "#/responses/notFound"

	id := ctx.ParamsInt64(":check_run_id")
	checkRun, err := git_model.GetCheckRunByRepoIDAndID(ctx, ctx.Repo.Repository.ID, id)
	if err != nil {
		if git_model.IsErrCheckRunNotExist(err) {
			ctx.NotFound(err)
			return
		}

		ctx.Error(http.StatusInternalServerError, "GetCheckRunByRepoIDAndID", err)
		return
	}

	err = checkRun.LoadOutput(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "checkRun.LoadOutput", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToChekckRun(ctx, checkRun))
}

// UpdateCheckRun Update a check run
func UpdateCheckRun(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/check-runs/{check_run_id} checkRuns checkRunsUpdate
	// ---
	// summary: Update a check run
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
	// - name: check_run_id
	//   in: path
	//   description: id of the check run
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateCheckRunOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/CheckRun"
	//   "404":
	//     "$ref": "#/responses/notFound"
	id := ctx.ParamsInt64(":check_run_id")
	checkRun, err := git_model.GetCheckRunByRepoIDAndID(ctx, ctx.Repo.Repository.ID, id)
	if err != nil {
		if git_model.IsErrCheckRunNotExist(err) {
			ctx.NotFound(err)
			return
		}

		ctx.Error(http.StatusInternalServerError, "GetCheckRunByRepoIDAndID", err)
		return
	}

	form := web.GetForm(ctx).(*api.UpdateCheckRunOptions)
	opts := git_model.UpdateCheckRunOptions{
		Creator:    ctx.Doer,
		Repo:       ctx.Repo.Repository,
		ExternalID: form.ExternalID,
		DetailsURL: form.DetailsURL,
		Output:     form.Output,
	}
	if form.Name != nil {
		opts.Name = *form.Name
	}

	err = files_service.LoadPatchsForCheckRunOutput(ctx, ctx.Repo.Repository, checkRun.HeadSHA, opts.Output)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadPatchsForCheckRunOutput", err)
		return
	}

	if form.StartedAt != nil {
		opts.StartedAt = timeutil.TimeStamp(form.StartedAt.Unix())
	}

	if form.CompletedAt != nil {
		opts.CompletedAt = timeutil.TimeStamp(form.CompletedAt.Unix())
	}

	if form.Status != nil {
		opts.Status = *form.Status
	}

	if form.Conclusion != nil {
		opts.Conclusion = *form.Conclusion
	}

	err = checkRun.Update(ctx, opts)
	if err != nil {
		if git_model.IsErrCheckRunExist(err) || git_model.IsErrUnVaildCheckRunOptions(err) {
			ctx.Error(http.StatusBadRequest, "UpdateCheckRun", err)
			return
		}

		ctx.Error(http.StatusInternalServerError, "UpdateCheckRun", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToChekckRun(ctx, checkRun))
}

// ListCheckRun List check runs for a Git reference
func ListCheckRun(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{ref}/check-runs checkRuns checkRunsList
	// ---
	// summary: ListCheckRun List check runs for a Git reference
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
	// - name: ref
	//   in: path
	//   description: name of branch/tag/commit
	//   type: string
	//   required: true
	// - name: sort
	//   in: query
	//   description: type of sort
	//   type: string
	//   enum: [oldest, recentupdate, leastupdate]
	//   required: false
	// - name: status
	//   in: query
	//   description: type of status
	//   type: string
	//   enum: [queued, in_progress, completed]
	//   required: false
	// - name: conclusion
	//   in: query
	//   description: type of conclusion
	//   type: string
	//   enum: [action_required, cancelled, failure, neutral, success, skipped, stale, timed_out]
	//   required: false
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommitStatusList"

	filter := utils.ResolveRefOrSha(ctx, ctx.Params("ref"))
	if ctx.Written() {
		return
	}

	getCommitCheckRuns(ctx, filter) // By default filter is maybe the raw SHA
}

func getCommitCheckRuns(ctx *context.APIContext, sha string) {
	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "ref/sha not given", nil)
		return
	}
	sha = utils.MustConvertToSHA1(ctx.Context, sha)
	repo := ctx.Repo.Repository

	listOptions := utils.GetListOptions(ctx)

	checkRuns, maxResults, err := git_model.GetCheckRuns(ctx, repo, sha, &git_model.CheckRunOptions{
		ListOptions: listOptions,
		SortType:    ctx.FormTrim("sort"),
		Status:      ctx.FormTrim("status"),
		Conclusion:  ctx.FormTrim("conclusion"),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCheckRuns", fmt.Errorf("GetCheckRuns[%s, %s, %d]: %w", repo.FullName(), sha, ctx.FormInt("page"), err))
		return
	}

	apiCheckRuns := make([]*api.CheckRun, 0, len(checkRuns))
	for _, checkRun := range checkRuns {
		apiCheckRuns = append(apiCheckRuns, convert.ToChekckRun(ctx, checkRun))
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)

	ctx.JSON(http.StatusOK, apiCheckRuns)
}
