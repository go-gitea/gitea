package private

import (
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	macaron "gopkg.in/macaron.v1"
)

// GetRepository return the default branch of a repository
func GetRepository(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":rid")
	repository, err := models.GetRepositoryByID(repoID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, repository)
}

// GetActivePullRequest return an active pull request when it exists or an empty object
func GetActivePullRequest(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":rid")

	infoPath, err := url.QueryUnescape(ctx.Params("*"))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	infos := strings.Split(infoPath, "...")

	pr, err := models.GetUnmergedPullRequest(repoID, repoID, infos[1], infos[0])
	if err != nil && !models.IsErrPullRequestNotExist(err) {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, pr)
}
