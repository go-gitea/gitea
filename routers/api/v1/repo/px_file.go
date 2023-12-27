package repo

import (
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	files_service "code.gitea.io/gitea/services/repository/files"
)

// GetCommitsContents Get the metadata and commit and contents (if a file) of an entry in a repository, or a list of entries if a dir
func GetCommitsContents(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commit_contents/commits repository repoGetContentsList
	// ---
	// summary: Gets the metadata of all the entries of the root dir
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
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContentsListResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// same as GetContents(), this function is here because swagger fails if path is empty in GetContents() interface
	if !canReadFiles(ctx.Repo) {
		ctx.Error(http.StatusInternalServerError, "GetContentsOrList", repo_model.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.Doer.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		})
		return
	}

	treePath := ctx.Params("*")
	ref := ctx.FormTrim("ref")

	if fileList, err := files_service.GetCommitContentsOrList(ctx, ctx.Repo.Repository, treePath, ref); err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetContentsOrList", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetContentsOrList", err)
	} else {
		ctx.JSON(http.StatusOK, fileList)
	}

}

// GetCommitsContentsList Get the metadata (include commit information) of all the entries of the root dir
func GetCommitsContentsList(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commit_contents repository repoGetContentsList
	// ---
	// summary: Gets the metadata of all the entries of the root dir
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
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContentsListResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// same as GetContents(), this function is here because swagger fails if path is empty in GetContents() interface
	GetCommitsContents(ctx)

}
